package fasitd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/fasitd/commandstatus"
	"github.com/nais/fasit/internal/fasitd/fasitdsql"
	"github.com/nais/fasit/internal/fasitd/protogen"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const sendBuffer = 64

// Server implements the fasitd gRPC session protocol. It accepts long-lived
// bidirectional streams, tracks active sessions in memory, and persists the
// reports agents send back into the fasitd_* tables.
type Server struct {
	protogen.UnimplementedFasitdServer

	pool        *pgxpool.Pool
	registry    *registry
	querier     fasitdsql.Querier
	loadContext contextloader.LoaderFunc
	log         *slog.Logger

	reportsRecv metric.Int64Counter
}

func NewServer(pool *pgxpool.Pool, loadContext contextloader.LoaderFunc, meter metric.Meter, log *slog.Logger) (*Server, error) {
	reportsRecv, err := meter.Int64Counter(
		"fasitd_reports_received_total",
		metric.WithDescription("Total reports received from fasitd agents, by type"),
	)
	if err != nil {
		return nil, fmt.Errorf("create reports counter: %w", err)
	}

	return &Server{
		pool:        pool,
		registry:    newRegistry(),
		querier:     fasitdsql.New(pool),
		loadContext: loadContext,
		log:         log.With("subsystem", "fasitd-server"),
		reportsRecv: reportsRecv,
	}, nil
}

// Registry exposes the session registry so the deployer can route commands.
func (s *Server) Registry() *registry { return s.registry }

func (s *Server) Connect(stream protogen.Fasitd_ConnectServer) (err error) {
	ctx := s.loadContext(stream.Context())

	first, err := stream.Recv()
	if err != nil {
		return err
	}
	reg := first.GetRegister()
	if reg == nil {
		return status.Error(codes.InvalidArgument, "first message must be Register")
	}
	if reg.GetTenant() == "" || reg.GetEnvironment() == "" {
		return status.Error(codes.InvalidArgument, "register requires tenant and environment")
	}
	if err := authorizeRegister(ctx, reg.GetTenant(), reg.GetEnvironment()); err != nil {
		return err
	}

	env, err := resolveEnvironment(ctx, reg.GetTenant(), reg.GetEnvironment())
	if err != nil {
		return status.Errorf(codes.NotFound, "unknown tenant/environment: %v", err)
	}

	sess := &session{
		env:           env,
		fasitdVersion: reg.GetFasitdVersion(),
		send:          make(chan *protogen.ServerMessage, sendBuffer),
		done:          make(chan struct{}),
		stats:         newSessionStats(reg.GetInstance(), time.Now()),
	}
	sess.stats.recordIn(KindRegister, "version "+reg.GetFasitdVersion(), time.Now())

	log := s.log.With("tenant", env.Tenant, "environment", env.Environment, "fasitd_version", reg.GetFasitdVersion(), "instance", reg.GetInstance())

	s.registry.track(sess)
	n := s.registry.add(sess)
	defer func() {
		s.retire(context.WithoutCancel(ctx), sess, "fasitd session closed before delivery")
		close(sess.done)
		s.registry.untrack(sess, closeReason(err), time.Now())
	}()

	log.With("active_sessions", n).Info("fasitd session connected")
	defer log.Info("fasitd session disconnected")

	sendErr := make(chan error, 1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-sess.done:
				return
			case msg := <-sess.send:
				if !sess.accepting() {
					s.markUndeliverable(ctx, msg, "fasitd session draining")
					sess.stats.recordDropped(KindUndeliverable, commandSummary(msg.GetCommand()), time.Now())
					continue
				}
				if err := stream.Send(msg); err != nil {
					// Stop routing to this session at once so later commands fail
					// over; the receive loop only notices on its next message.
					s.markUndeliverable(context.WithoutCancel(ctx), msg, "fasitd session send failed")
					sess.stats.recordDropped(KindUndeliverable, commandSummary(msg.GetCommand()), time.Now())
					s.retire(context.WithoutCancel(ctx), sess, "fasitd session send failed")
					sendErr <- err
					return
				}
				cmd := msg.GetCommand()
				sess.stats.rememberCommand(cmd.GetDiid(), commandLabel(cmd))
				sess.stats.recordOut(KindCommand, commandSummary(cmd), time.Now())
			}
		}
	}()

	for {
		msg, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		select {
		case err := <-sendErr:
			return err
		default:
		}
		if err := s.handleAgentMessage(ctx, sess, msg); err != nil {
			log.With("err", err).Error("handle agent message")
		}
	}
}

// retire removes sess from command routing and records any commands still
// queued for it as undeliverable.
func (s *Server) retire(ctx context.Context, sess *session, reason string) {
	s.registry.remove(sess)
	for _, msg := range sess.stopAccepting() {
		s.markUndeliverable(ctx, msg, reason)
		sess.stats.recordDropped(KindUndeliverable, commandSummary(msg.GetCommand()), time.Now())
	}
}

func closeReason(err error) string {
	switch {
	case err == nil:
		return "closed by fasitd"
	case status.Code(err) == codes.Canceled || errors.Is(err, context.Canceled):
		return "stream cancelled"
	default:
		return err.Error()
	}
}

func (s *Server) handleAgentMessage(ctx context.Context, sess *session, msg *protogen.AgentMessage) error {
	now := time.Now()
	switch m := msg.GetMessage().(type) {
	case *protogen.AgentMessage_Ack:
		sess.stats.recordIn(KindAck, sess.stats.commandLabel(m.Ack.GetDiid(), false), now)
		s.countReport(ctx, KindAck)
		return s.appendStatus(ctx, m.Ack.GetDiid(), string(commandstatus.Installing), "")
	case *protogen.AgentMessage_Status:
		summary := m.Status.GetStatus() + " " + sess.stats.commandLabel(m.Status.GetDiid(), true)
		if e := m.Status.GetError(); e != "" {
			summary += ": " + e
		}
		sess.stats.recordIn(KindStatus, summary, now)
		s.countReport(ctx, KindStatus)
		return s.appendStatus(ctx, m.Status.GetDiid(), m.Status.GetStatus(), m.Status.GetError())
	case *protogen.AgentMessage_Logs:
		sess.stats.recordIn(KindLogs, fmt.Sprintf("%d lines for %s", len(m.Logs.GetLines()), sess.stats.commandLabel(m.Logs.GetDiid(), false)), now)
		s.countReport(ctx, KindLogs)
		return s.appendLogs(ctx, m.Logs)
	case *protogen.AgentMessage_Releases:
		s.countReport(ctx, KindReleases)
		summary := fmt.Sprintf("%d releases", len(m.Releases.GetReleases()))
		if !s.registry.isPrimary(sess) {
			sess.stats.recordIn(KindReleases, summary+" (ignored, not primary)", now)
			return nil
		}
		sess.stats.recordIn(KindReleases, summary, now)
		if err := s.setReleases(ctx, sess.env.ID, m.Releases); err != nil {
			return err
		}
		s.registry.recordReleasesReport(sess.env.ID, now)
		return nil
	case *protogen.AgentMessage_Draining:
		sess.stats.recordIn(KindDraining, "", now)
		s.countReport(ctx, KindDraining)
		sess.stats.markDraining(now)
		s.retire(ctx, sess, "fasitd session draining")
		s.log.With("tenant", sess.env.Tenant, "environment", sess.env.Environment, "fasitd_version", sess.fasitdVersion).Info("fasitd session draining")
		return nil
	case *protogen.AgentMessage_Register:
		sess.stats.recordIn(KindRegister, "duplicate", now)
		return status.Error(codes.InvalidArgument, "duplicate register message")
	case *protogen.AgentMessage_Heartbeat:
		sess.stats.recordIn(KindHeartbeat, "", now)
		return nil
	default:
		sess.stats.recordIn(KindUnknown, "", now)
		s.log.Warn("unknown agent message")
		return nil
	}
}
