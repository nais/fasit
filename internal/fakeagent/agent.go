// Package fakeagent provides an in-process stand-in for naisd used in local
// development. It replaces the Pub/Sub round-trip between fasit and naisd:
// deploy instructions from the reconciler are "executed" against an in-memory
// Helm state, and the resulting deploy-status, release-state and health
// messages are fed straight back to the reconciler's receiver. No Pub/Sub
// emulator or separate naisd process is required.
package fakeagent

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	envpkg "github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/reconciler"
)

type envKey struct {
	tenant string
	env    string
}

type releaseState struct {
	version      string
	revision     int
	status       string
	lastDeployed time.Time
}

// Options configures which environments deviate from the happy path, mirroring
// the demo mix that mise/tasks/naisd-all.sh used to produce.
type Options struct {
	// FailingEnvs (each "tenant/env") deploy but report a failed Helm rollout.
	FailingEnvs []string
	// UnhealthyEnvs (each "tenant/env") never report health, so the reconciler
	// skips them and their features stay PENDING. Mirrors a cluster where no
	// naisd is running.
	UnhealthyEnvs []string
}

type Agent struct {
	log           *slog.Logger
	deployDelay   time.Duration
	failingEnvs   map[envKey]bool
	unhealthyEnvs map[envKey]bool

	out chan message.Status

	mu       sync.Mutex
	releases map[envKey]map[string]*releaseState
}

func New(log *slog.Logger, opts Options) *Agent {
	return &Agent{
		log:           log,
		deployDelay:   2 * time.Second,
		failingEnvs:   parseEnvSet(opts.FailingEnvs),
		unhealthyEnvs: parseEnvSet(opts.UnhealthyEnvs),
		out:           make(chan message.Status, 256),
		releases:      map[envKey]map[string]*releaseState{},
	}
}

func parseEnvSet(items []string) map[envKey]bool {
	m := map[envKey]bool{}
	for _, item := range items {
		t, e, ok := strings.Cut(strings.TrimSpace(item), "/")
		if !ok {
			continue
		}
		m[envKey{strings.TrimSpace(t), strings.TrimSpace(e)}] = true
	}
	return m
}

// Publisher returns a reconciler.Publisher bound to the tenant/env encoded in
// topicID. It satisfies reconciler.NewPublisher, so the existing pubSubDeployer
// drives it unchanged.
func (a *Agent) Publisher(topicID string, _ *slog.Logger) reconciler.Publisher {
	tenant, env := parseTopic(topicID)
	return &publisher{agent: a, key: envKey{tenant, env}}
}

type publisher struct {
	agent *Agent
	key   envKey
}

func (p *publisher) Stop() {}

func (p *publisher) Publish(_ context.Context, instr message.DeployInstruction) error {
	p.agent.handleInstruction(p.key, instr)
	return nil
}

// parseTopic reverses reconciler's "naisd-<tenant>-<env>" topic naming. Env
// names contain no dashes, so the tenant is everything before the last dash.
func parseTopic(topicID string) (tenant, env string) {
	s := strings.TrimPrefix(topicID, "naisd-")
	i := strings.LastIndex(s, "-")
	if i < 0 {
		return s, ""
	}
	return s[:i], s[i+1:]
}

func (a *Agent) handleInstruction(key envKey, instr message.DeployInstruction) {
	// Emit asynchronously after a short delay: the deployer writes the "sent"
	// deploy_log row only after Publish returns, and the receiver looks the
	// instruction up by DIID, so the status must arrive once that row exists.
	go func() {
		time.Sleep(a.deployDelay)

		if instr.Uninstall {
			a.recordUninstall(key, instr.Name)
			a.emitReleases(key)
			return
		}

		failing := a.failingEnvs[key]
		a.recordUpgrade(key, instr, failing)
		a.emitHelm(key, instr, feature.DeployStatusInstalling, "")
		a.emitLog(key, instr, failing)
		if failing {
			a.emitHelm(key, instr, feature.DeployStatusFailed, "helm upgrade failed")
		} else {
			a.emitHelm(key, instr, feature.DeployStatusDeployed, "")
		}
		a.emitReleases(key)
	}()
}

func (a *Agent) recordUpgrade(key envKey, instr message.DeployInstruction, failing bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	envRel := a.releases[key]
	if envRel == nil {
		envRel = map[string]*releaseState{}
		a.releases[key] = envRel
	}
	rel := envRel[instr.Name]
	if rel == nil {
		rel = &releaseState{}
		envRel[instr.Name] = rel
	}
	rel.version = instr.Version
	rel.revision++
	rel.lastDeployed = time.Now()
	if failing {
		rel.status = "failed"
	} else {
		rel.status = "deployed"
	}
}

func (a *Agent) recordUninstall(key envKey, name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if envRel := a.releases[key]; envRel != nil {
		delete(envRel, name)
	}
}

func (a *Agent) emitHelm(key envKey, instr message.DeployInstruction, status feature.DeployStatus, errMsg string) {
	a.send(key, message.StatusTypeHelm, message.Helm{
		DIID:          instr.ID,
		Name:          instr.Name,
		Version:       instr.Version,
		RolloutStatus: status,
		ConfigHash:    instr.ConfigHash,
		Error:         errMsg,
	})
}

func (a *Agent) emitLog(key envKey, instr message.DeployInstruction, failing bool) {
	lines := mockDeployLog
	if failing {
		lines = mockFailLog
	}
	now := time.Now()
	logs := make([]message.LogLine, len(lines))
	for i, msg := range lines {
		logs[i] = message.LogLine{Time: now.Add(time.Duration(i) * time.Millisecond), Msg: msg}
	}
	a.send(key, message.StatusTypeLog, message.StatusLog{DIID: instr.ID, Logs: logs})
}

func (a *Agent) emitReleases(key envKey) {
	a.mu.Lock()
	envRel := a.releases[key]
	releases := make([]message.Release, 0, len(envRel))
	for name, rel := range envRel {
		releases = append(releases, message.Release{
			Name:         name,
			Version:      rel.version,
			Status:       rel.status,
			Revision:     rel.revision,
			LastDeployed: rel.lastDeployed,
		})
	}
	a.mu.Unlock()

	a.send(key, message.StatusTypeHelmReleases, message.HelmRelease{
		Created:  time.Now(),
		Releases: releases,
	})
}

func (a *Agent) send(key envKey, t message.StatusType, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		a.log.With("err", err).Error("marshal status")
		return
	}
	a.out <- message.Status{Tenant: key.tenant, Environment: key.env, Type: t, Data: data}
}

// Receive satisfies reconciler.ReceiverClient, delivering the statuses produced
// by the fake agent to the reconciler's receiver.
func (a *Agent) Receive(ctx context.Context, f func(ctx context.Context, msg message.Status) error) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg := <-a.out:
			if err := f(ctx, msg); err != nil {
				a.log.With("err", err, "type", msg.Type.String()).Error("handle status")
			}
		}
	}
}

// ReportHealth periodically reports every (reconciled) environment as healthy,
// except those configured unhealthy. Without this the reconciler treats every
// environment as having no naisd and refuses to deploy. ctx must carry the
// context-loader queriers.
func (a *Agent) ReportHealth(ctx context.Context, interval time.Duration) {
	first := true
	for {
		envs, err := envpkg.ListTenantEnvironments(ctx, false)
		if err != nil {
			a.log.With("err", err).Warn("list environments for health")
		} else {
			for _, e := range envs {
				key := envKey{e.TenantName, e.Name}
				if a.unhealthyEnvs[key] {
					continue
				}
				a.send(key, message.StatusTypeHealth, message.Health{ReportedAt: time.Now()})
			}
			if first {
				first = false
				// Nudge the reconciler so deploys don't wait a full cycle for the
				// freshly-reported health to be persisted.
				time.AfterFunc(interval/2, reconciler.TriggerReconcile)
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

var mockDeployLog = []string{
	"Pulled: europe-north1-docker.pkg.dev/nais-io/nais/feature/mock:local",
	"Digest: sha256:dde4e99c363e53fea798b642e25fdfe6407a6cc1091dc9eaf9e68b04794e03fe",
	"Release has been upgraded. Happy Helming!",
}

var mockFailLog = []string{
	"Pulled: europe-north1-docker.pkg.dev/nais-io/nais/feature/mock:local",
	"pod (v1Pod): Back-off restarting failed container",
	"pod (v1Pod): Readiness probe failed: HTTP probe failed with statuscode: 503",
	"Error: UPGRADE FAILED: timed out waiting for the condition",
}
