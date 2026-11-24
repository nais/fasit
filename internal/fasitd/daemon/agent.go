package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

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
		go a.reportReleasesLoop(ctx, send)
	}

	var sendDrainingOnce sync.Once
	sendDraining := func() {
		sendDrainingOnce.Do(func() {
			if err := send(&protogen.AgentMessage{
				Message: &protogen.AgentMessage_Draining{Draining: &protogen.Draining{}},
			}); err != nil {
				a.log.With("err", err).Warn("send draining")
			}
		})
	}

	var cmdMu sync.Mutex
	inFlight := false
	cmdDone := make(chan struct{}, 1)

	go func() {
		<-ctx.Done()
		a.ready.Store(false)
		sendDraining()
		cmdMu.Lock()
		busy := inFlight
		cmdMu.Unlock()
		if busy {
			<-cmdDone
		}
		cancelStream()
	}()

	for {
		msg, err := stream.Recv()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("recv: %w", err)
		}
		cmd := msg.GetCommand()
		if cmd == nil {
			a.log.Warn("ignoring non-command server message")
			continue
		}

		cmdMu.Lock()
		inFlight = true
		cmdMu.Unlock()

		// Commands are handled serially: one environment, one in-flight command.
		draining, err := a.handleCommand(streamCtx, send, sendDraining, cmd)
		if err != nil {
			a.log.With("err", err, "feature", cmd.GetName()).Error("handle command")
		}

		cmdMu.Lock()
		inFlight = false
		cmdMu.Unlock()
		select {
		case cmdDone <- struct{}{}:
		default:
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// If a self-upgrade failed and rolled back, this pod stays alive; close
		// the drained stream so the outer loop reconnects and re-registers.
		if draining {
			return nil
		}
	}
}

func (a *Agent) handleCommand(ctx context.Context, send func(*protogen.AgentMessage) error, sendDraining func(), cmd *protogen.Command) (bool, error) {
	diid := cmd.GetDiid()
	log := a.log.With("feature", cmd.GetName(), "version", cmd.GetVersion(), "diid", diid)

	if err := send(ack(diid)); err != nil {
		return false, err
	}

	if a.opts.DryRun {
		return false, a.handleDryRun(send, cmd, log)
	}

	if a.opts.Executor == nil {
		return false, send(statusMsg(diid, "failed", cmd.GetConfigHash(), "no helm executor configured"))
	}

	if cmd.GetUninstall() && cmd.GetName() == selfReleaseName {
		return false, send(statusMsg(diid, "failed", cmd.GetConfigHash(), "refusing to uninstall self"))
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
			return false, send(statusMsg(diid, "failed", cmd.GetConfigHash(), err.Error()))
		}
		return false, send(statusMsg(diid, "deployed", cmd.GetConfigHash(), ""))
	}

	vals, err := parseValues(cmd.GetValues())
	if err != nil {
		log.With("err", err).Error("parse command values")
		return false, send(statusMsg(diid, "failed", cmd.GetConfigHash(), err.Error()))
	}

	// Self-upgrades always run atomically: if the new fasitd pod fails its
	// /readyz probe (cannot connect and register with Fasit), Helm rolls back
	// to the previous revision automatically. Signal Draining first so Fasit
	// routes subsequent commands to the next replica while this pod upgrades.
	atomicUpgrade := cmd.GetName() == selfReleaseName
	if atomicUpgrade {
		sendDraining()
	}

	log.With("atomic", atomicUpgrade).Info("deploying release")
	err = a.opts.Executor.InstallOrUpgrade(ctx, DeployRequest{
		Name:    cmd.GetName(),
		Chart:   cmd.GetChart(),
		Version: cmd.GetVersion(),
		Values:  vals,
		Timeout: timeout,
		Atomic:  atomicUpgrade,
	}, logFn)
	if err != nil {
		log.With("err", err).Error("helm deploy failed")
		return atomicUpgrade, send(statusMsg(diid, "failed", cmd.GetConfigHash(), err.Error()))
	}

	return atomicUpgrade, send(statusMsg(diid, "deployed", cmd.GetConfigHash(), ""))
}

func (a *Agent) handleDryRun(send func(*protogen.AgentMessage) error, cmd *protogen.Command, log *slog.Logger) error {
	diid := cmd.GetDiid()
	verb := "deploy"
	if cmd.GetUninstall() {
		verb = "uninstall"
	}
	log.With("uninstall", cmd.GetUninstall()).Info("dry-run command received")

	if err := send(logBatch(diid, fmt.Sprintf("dry-run: would %s %s version %s (no helm executed)", verb, cmd.GetName(), cmd.GetVersion()))); err != nil {
		return err
	}

	return send(statusMsg(diid, "deployed", cmd.GetConfigHash(), ""))
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

func statusMsg(diid, status, hash, errMsg string) *protogen.AgentMessage {
	return &protogen.AgentMessage{Message: &protogen.AgentMessage_Status{Status: &protogen.CommandStatus{
		Diid:       diid,
		Status:     status,
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
