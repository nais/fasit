package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nais/fasit/internal/fasitd/commandstatus"
	"github.com/nais/fasit/internal/fasitd/protogen"
	"google.golang.org/grpc/metadata"
	"helm.sh/helm/v3/pkg/release"
)

const (
	protocolVersion = 1
	selfReleaseName = "fasitd"
)

// heartbeatInterval must stay comfortably below the smallest idle timeout of any
// intermediary on the path (haproxy timeout-client defaults to 50s).
const heartbeatInterval = 20 * time.Second

// defaultDrainGrace is how long a drained session keeps reading before it
// closes, so commands Fasit sent just before it processed Draining are rejected
// as undeliverable instead of being lost on the wire.
const defaultDrainGrace = 2 * time.Second

// ReleaseLister lists the helm releases installed in the agent's environment.
type ReleaseLister interface {
	List() ([]*release.Release, error)
}

// AgentOptions configures a fasitd agent.
type AgentOptions struct {
	Tenant        string
	Environment   string
	Version       string
	ReleaseLister ReleaseLister
	Executor      Executor
	// ReleaseInterval mirrors naisd's helm-list cadence. Zero disables periodic
	// release reporting.
	ReleaseInterval time.Duration
	// DryRun reports what would happen without executing Helm.
	DryRun bool
	// DrainGrace overrides defaultDrainGrace.
	DrainGrace time.Duration
}

// Agent runs the client side of a fasitd session. When DryRun is true it never
// executes Helm: it acknowledges commands and reports the lifecycle as if the
// deploy succeeded. When DryRun is false it runs Helm via Executor; self-upgrades
// (release name "fasitd") run with Atomic=true so Helm rolls back automatically
// if the new pod fails its readiness probe.
type Agent struct {
	client protogen.FasitdClient
	opts   AgentOptions
	log    *slog.Logger

	ready atomic.Bool
}

func NewAgent(client protogen.FasitdClient, opts AgentOptions, log *slog.Logger) *Agent {
	return &Agent{client: client, opts: opts, log: log.With("subsystem", "fasitd-agent")}
}

// Ready reports whether the agent currently has an active, registered session
// with Fasit. Used by the /readyz probe so Kubernetes rolling updates wait for
// the new pod to connect and take over before terminating the old pod.
func (a *Agent) Ready() bool {
	return a.ready.Load()
}

// Run opens the session and blocks until the stream closes or ctx is cancelled.
// When ctx is cancelled while a command is in flight (e.g. SIGTERM during a
// self-upgrade once the new pod passes /readyz), the stream stays open until
// the command finishes and its terminal status has been sent.
func (a *Agent) Run(ctx context.Context) error {
	streamCtx, cancelStream := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelStream()
	defer a.ready.Store(false)

	streamCtx = metadata.AppendToOutgoingContext(
		streamCtx,
		"x-fasit-tenant", a.opts.Tenant,
		"x-fasit-environment", a.opts.Environment,
	)
	stream, err := a.client.Connect(streamCtx)
	if err != nil {
		return fmt.Errorf("open session: %w", err)
	}

	var sendMu sync.Mutex
	send := func(msg *protogen.AgentMessage) error {
		sendMu.Lock()
		defer sendMu.Unlock()
		return stream.Send(msg)
	}

	if err := send(&protogen.AgentMessage{
		Message: &protogen.AgentMessage_Register{
			Register: &protogen.Register{
				Tenant:          a.opts.Tenant,
				Environment:     a.opts.Environment,
				FasitdVersion:   a.opts.Version,
				ProtocolVersion: protocolVersion,
			},
		},
	}); err != nil {
		return fmt.Errorf("register: %w", err)
	}
	a.ready.Store(true)
	a.log.With("tenant", a.opts.Tenant, "environment", a.opts.Environment).Info("fasitd session registered")

	go a.heartbeatLoop(streamCtx, send)

	if a.opts.ReleaseLister != nil && a.opts.ReleaseInterval > 0 {
		go a.reportReleasesLoop(streamCtx, send)
	}

	var draining atomic.Bool
	var drainOnce sync.Once
	drain := func() {
		drainOnce.Do(func() {
			draining.Store(true)
			if err := send(&protogen.AgentMessage{
				Message: &protogen.AgentMessage_Draining{Draining: &protogen.Draining{}},
			}); err != nil {
				a.log.With("err", err).Warn("send draining")
			}
		})
	}

	grace := a.opts.DrainGrace
	if grace <= 0 {
		grace = defaultDrainGrace
	}
	closeAfterGrace := func() { time.AfterFunc(grace, cancelStream) }

	gate := &commandGate{}
	go func() {
		select {
		case <-streamCtx.Done():
			return
		case <-ctx.Done():
		}
		a.ready.Store(false)
		drain()
		<-gate.stop()
		closeAfterGrace()
	}()

	reconnecting := false
	for {
		msg, err := stream.Recv()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if reconnecting {
				return nil
			}
			return fmt.Errorf("recv: %w", err)
		}
		cmd := msg.GetCommand()
		if cmd == nil {
			a.log.Warn("ignoring non-command server message")
			continue
		}

		finish, ok := gate.begin()
		if !ok || draining.Load() {
			if finish != nil {
				finish()
			}
			if err := send(statusMsg(cmd.GetDiid(), commandstatus.Undeliverable, cmd.GetConfigHash(), "fasitd draining")); err != nil {
				a.log.With("err", err, "feature", cmd.GetName()).Warn("reject command while draining")
			}
			continue
		}

		// Commands are handled serially: one environment, one in-flight command.
		reconnect, err := a.handleCommand(streamCtx, send, drain, cmd)
		finish()
		if err != nil {
			a.log.With("err", err, "feature", cmd.GetName()).Error("handle command")
		}
		if reconnect && !reconnecting && ctx.Err() == nil {
			reconnecting = true
			closeAfterGrace()
		}
	}
}

// commandGate coordinates command execution with shutdown: once stop is called
// no new command may begin, and stop's channel closes when the running command
// (if any) has finished.
type commandGate struct {
	mu       sync.Mutex
	stopping bool
	running  chan struct{}
}

func (g *commandGate) begin() (finish func(), ok bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.stopping {
		return nil, false
	}
	done := make(chan struct{})
	g.running = done
	return func() {
		g.mu.Lock()
		g.running = nil
		g.mu.Unlock()
		close(done)
	}, true
}

func (g *commandGate) stop() <-chan struct{} {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.stopping = true
	if g.running != nil {
		return g.running
	}
	idle := make(chan struct{})
	close(idle)
	return idle
}

// handleCommand executes cmd and reports its lifecycle. reconnect is true after
// a self-upgrade: this session was drained for it, so the caller must open a
// fresh session (if the pod is not being replaced) to receive commands again.
func (a *Agent) handleCommand(ctx context.Context, send func(*protogen.AgentMessage) error, drain func(), cmd *protogen.Command) (reconnect bool, err error) {
	diid := cmd.GetDiid()
	hash := cmd.GetConfigHash()
	log := a.log.With("feature", cmd.GetName(), "version", cmd.GetVersion(), "diid", diid)

	if err := send(ack(diid)); err != nil {
		return false, err
	}

	if a.opts.DryRun {
		return false, a.handleDryRun(send, cmd, log)
	}

	if a.opts.Executor == nil {
		return false, send(statusMsg(diid, commandstatus.Failed, hash, "no helm executor configured"))
	}

	if cmd.GetUninstall() && cmd.GetName() == selfReleaseName {
		return false, send(statusMsg(diid, commandstatus.Failed, hash, "refusing to uninstall self"))
	}

	logFn := func(msg string) {
		if err := send(logBatch(diid, msg)); err != nil {
			log.With("err", err).Warn("send helm log")
		}
	}

	timeout := time.Duration(cmd.GetTimeoutMs()) * time.Millisecond

	if cmd.GetUninstall() {
		log.Info("uninstalling release")
		if err := a.opts.Executor.Uninstall(ctx, cmd.GetName(), timeout, logFn); err != nil {
			log.With("err", err).Error("helm uninstall failed")
			return false, send(statusMsg(diid, commandstatus.Failed, hash, err.Error()))
		}
		return false, send(statusMsg(diid, commandstatus.Uninstalled, hash, ""))
	}

	vals, err := parseValues(cmd.GetValues())
	if err != nil {
		log.With("err", err).Error("parse command values")
		return false, send(statusMsg(diid, commandstatus.Failed, hash, err.Error()))
	}

	// Self-upgrades run atomically: if the new fasitd pod fails its /readyz
	// probe, Helm rolls back automatically. Draining first makes Fasit route
	// subsequent commands to the next session while this pod upgrades.
	selfUpgrade := cmd.GetName() == selfReleaseName
	if selfUpgrade {
		drain()
	}

	log.With("atomic", selfUpgrade).Info("deploying release")
	err = a.opts.Executor.InstallOrUpgrade(ctx, DeployRequest{
		Name:    cmd.GetName(),
		Chart:   cmd.GetChart(),
		Version: cmd.GetVersion(),
		Values:  vals,
		Timeout: timeout,
		Atomic:  selfUpgrade,
	}, logFn)
	if err != nil {
		log.With("err", err).Error("helm deploy failed")
		return selfUpgrade, send(statusMsg(diid, commandstatus.Failed, hash, err.Error()))
	}

	return selfUpgrade, send(statusMsg(diid, commandstatus.Deployed, hash, ""))
}

func (a *Agent) handleDryRun(send func(*protogen.AgentMessage) error, cmd *protogen.Command, log *slog.Logger) error {
	diid := cmd.GetDiid()
	verb, outcome := "deploy", commandstatus.Deployed
	if cmd.GetUninstall() {
		verb, outcome = "uninstall", commandstatus.Uninstalled
	}
	log.With("uninstall", cmd.GetUninstall()).Info("dry-run command received")

	if err := send(logBatch(diid, fmt.Sprintf("dry-run: would %s %s version %s (no helm executed)", verb, cmd.GetName(), cmd.GetVersion()))); err != nil {
		return err
	}

	return send(statusMsg(diid, outcome, cmd.GetConfigHash(), ""))
}

func (a *Agent) heartbeatLoop(ctx context.Context, send func(*protogen.AgentMessage) error) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := send(&protogen.AgentMessage{
				Message: &protogen.AgentMessage_Heartbeat{Heartbeat: &protogen.Heartbeat{}},
			}); err != nil {
				a.log.With("err", err).Warn("send heartbeat")
				return
			}
		}
	}
}

func (a *Agent) reportReleasesLoop(ctx context.Context, send func(*protogen.AgentMessage) error) {
	ticker := time.NewTicker(a.opts.ReleaseInterval)
	defer ticker.Stop()

	report := func() {
		releases, err := a.opts.ReleaseLister.List()
		if err != nil {
			a.log.With("err", err).Error("list releases")
			return
		}
		inv := &protogen.ReleaseInventory{CreatedUnix: time.Now().Unix()}
		for _, r := range releases {
			inv.Releases = append(inv.Releases, &protogen.Release{
				Name:             r.Name,
				Version:          r.Chart.Metadata.Version,
				Status:           r.Info.Status.String(),
				Revision:         int32(r.Version), // #nosec G115
				LastDeployedUnix: r.Info.LastDeployed.Unix(),
			})
		}
		if err := send(&protogen.AgentMessage{
			Message: &protogen.AgentMessage_Releases{Releases: inv},
		}); err != nil {
			a.log.With("err", err).Error("send releases")
		}
	}

	report()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			report()
		}
	}
}

func ack(diid string) *protogen.AgentMessage {
	return &protogen.AgentMessage{Message: &protogen.AgentMessage_Ack{Ack: &protogen.CommandAck{Diid: diid}}}
}

func statusMsg(diid string, status commandstatus.Status, hash, errMsg string) *protogen.AgentMessage {
	return &protogen.AgentMessage{Message: &protogen.AgentMessage_Status{Status: &protogen.CommandStatus{
		Diid:       diid,
		Status:     string(status),
		ConfigHash: hash,
		Error:      errMsg,
	}}}
}

func logBatch(diid, msg string) *protogen.AgentMessage {
	return &protogen.AgentMessage{Message: &protogen.AgentMessage_Logs{Logs: &protogen.LogBatch{
		Diid: diid,
		Lines: []*protogen.LogLine{{
			TimeUnixNano: time.Now().UnixNano(),
			Msg:          msg,
		}},
	}}}
}
