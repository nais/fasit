package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nais/fasit/internal/fasitd/protogen"
	"google.golang.org/grpc/metadata"
	"helm.sh/helm/v3/pkg/release"
)

const protocolVersion = 1

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
	// ReleaseInterval mirrors naisd's helm-list cadence. Zero disables periodic
	// release reporting.
	ReleaseInterval time.Duration
	// DryRun reports what would happen without executing Helm. Always true for now.
	DryRun bool
}

// Agent runs the client side of a fasitd session. In dry-run mode it never
// executes Helm: it acknowledges commands and reports the lifecycle as if the
// deploy succeeded, so Fasit can validate the transport and protocol alongside
// the canonical naisd path.
type Agent struct {
	client protogen.FasitdClient
	opts   AgentOptions
	log    *slog.Logger
}

func NewAgent(client protogen.FasitdClient, opts AgentOptions, log *slog.Logger) *Agent {
	return &Agent{client: client, opts: opts, log: log.With("subsystem", "fasitd-agent")}
}

// Run opens the session and blocks until the stream closes or ctx is cancelled.
func (a *Agent) Run(ctx context.Context) error {
	ctx = metadata.AppendToOutgoingContext(ctx,
		"x-fasit-tenant", a.opts.Tenant,
		"x-fasit-environment", a.opts.Environment,
	)
	stream, err := a.client.Connect(ctx)
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
	a.log.With("tenant", a.opts.Tenant, "environment", a.opts.Environment).Info("fasitd session registered")

	if a.opts.ReleaseLister != nil && a.opts.ReleaseInterval > 0 {
		go a.reportReleasesLoop(ctx, send)
	}

	for {
		msg, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("recv: %w", err)
		}
		cmd := msg.GetCommand()
		if cmd == nil {
			a.log.Warn("ignoring non-command server message")
			continue
		}
		// Commands are handled serially: one environment, one in-flight command.
		if err := a.handleCommand(send, cmd); err != nil {
			a.log.With("err", err, "feature", cmd.GetName()).Error("handle command")
		}
	}
}

func (a *Agent) handleCommand(send func(*protogen.AgentMessage) error, cmd *protogen.Command) error {
	diid := cmd.GetDiid()
	log := a.log.With("feature", cmd.GetName(), "version", cmd.GetVersion(), "diid", diid)

	if err := send(ack(diid)); err != nil {
		return err
	}

	verb := "deploy"
	if cmd.GetUninstall() {
		verb = "uninstall"
	}
	log.With("uninstall", cmd.GetUninstall()).Info("dry-run command received")

	if err := send(logBatch(diid, fmt.Sprintf("dry-run: would %s %s version %s (no helm executed)", verb, cmd.GetName(), cmd.GetVersion()))); err != nil {
		return err
	}

	// Dry-run: no Helm. Report terminal success meaning "dry-run succeeded".
	return send(statusMsg(diid, "deployed", cmd.GetConfigHash(), ""))
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
