package fasitd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/contextloader"
	envpkg "github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/fasitd/commandstatus"
	"github.com/nais/fasit/internal/fasitd/fasitdsql"
	"github.com/nais/fasit/internal/fasitd/protogen"
	"go.opentelemetry.io/otel/attribute"
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

func (s *Server) Connect(stream protogen.Fasitd_ConnectServer) error {
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

	envID, err := resolveEnvironmentID(ctx, reg.GetTenant(), reg.GetEnvironment())
	if err != nil {
		return status.Errorf(codes.NotFound, "unknown tenant/environment: %v", err)
	}

	sess := &session{
		key:           keyFor(reg.GetTenant(), reg.GetEnvironment()),
		environmentID: envID,
		fasitdVersion: reg.GetFasitdVersion(),
		send:          make(chan *protogen.ServerMessage, sendBuffer),
		done:          make(chan struct{}),
	}

	log := s.log.With("tenant", reg.GetTenant(), "environment", reg.GetEnvironment(), "fasitd_version", reg.GetFasitdVersion())

	n := s.registry.add(sess)
	defer func() {
		s.retire(context.WithoutCancel(ctx), sess, "fasitd session closed before delivery")
		close(sess.done)
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
					continue
				}
				if err := stream.Send(msg); err != nil {
					sendErr <- err
					return
				}
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

// resolveEnvironmentID looks up the environment ID for a tenant and environment
// name. ctx must carry the environment querier (see contextloader).
func resolveEnvironmentID(ctx context.Context, tenant, environment string) (uuid.UUID, error) {
	t, err := envpkg.GetTenantByName(ctx, tenant)
	if err != nil {
		return uuid.Nil, fmt.Errorf("get tenant %s: %w", tenant, err)
	}
	env, err := envpkg.GetByName(ctx, t.ID, environment)
	if err != nil {
		return uuid.Nil, fmt.Errorf("get environment %s/%s: %w", tenant, environment, err)
	}
	return env.ID, nil
}

// retire removes sess from command routing and records any commands still
// queued for it as undeliverable.
func (s *Server) retire(ctx context.Context, sess *session, reason string) {
	s.registry.remove(sess)
	for _, msg := range sess.stopAccepting() {
		s.markUndeliverable(ctx, msg, reason)
	}
}

func (s *Server) markUndeliverable(ctx context.Context, msg *protogen.ServerMessage, reason string) {
	diid := msg.GetCommand().GetDiid()
	if err := s.appendStatus(ctx, diid, string(commandstatus.Undeliverable), reason); err != nil {
		s.log.With("err", err, "diid", diid).Error("mark command undeliverable")
	}
}

func (s *Server) handleAgentMessage(ctx context.Context, sess *session, msg *protogen.AgentMessage) error {
	switch m := msg.GetMessage().(type) {
	case *protogen.AgentMessage_Ack:
		s.countReport(ctx, "ack")
		return s.appendStatus(ctx, m.Ack.GetDiid(), string(commandstatus.Installing), "")
	case *protogen.AgentMessage_Status:
		s.countReport(ctx, "status")
		return s.appendStatus(ctx, m.Status.GetDiid(), m.Status.GetStatus(), m.Status.GetError())
	case *protogen.AgentMessage_Logs:
		s.countReport(ctx, "logs")
		return s.appendLogs(ctx, m.Logs)
	case *protogen.AgentMessage_Releases:
		s.countReport(ctx, "releases")
		if !s.registry.isPrimary(sess) {
			return nil
		}
		return s.setReleases(ctx, sess.environmentID, m.Releases)
	case *protogen.AgentMessage_Draining:
		s.countReport(ctx, "draining")
		s.retire(ctx, sess, "fasitd session draining")
		s.log.With("tenant", sess.key.tenant, "environment", sess.key.environment, "fasitd_version", sess.fasitdVersion).Info("fasitd session draining")
		return nil
	case *protogen.AgentMessage_Register:
		return status.Error(codes.InvalidArgument, "duplicate register message")
	case *protogen.AgentMessage_Heartbeat:
		return nil
	default:
		s.log.Warn("unknown agent message")
		return nil
	}
}

func (s *Server) appendStatus(ctx context.Context, diidStr, statusStr, msg string) error {
	diid, err := uuid.Parse(diidStr)
	if err != nil {
		return fmt.Errorf("parse diid: %w", err)
	}
	if err := s.querier.AppendCommandStatus(ctx, fasitdsql.AppendCommandStatusParams{
		Diid:    diid,
		Status:  statusStr,
		Message: msg,
	}); err != nil {
		return fmt.Errorf("append command status: %w", err)
	}
	return nil
}

func (s *Server) appendLogs(ctx context.Context, batch *protogen.LogBatch) error {
	diid, err := uuid.Parse(batch.GetDiid())
	if err != nil {
		return fmt.Errorf("parse diid: %w", err)
	}
	lines := batch.GetLines()
	if len(lines) == 0 {
		return nil
	}
	params := make([]fasitdsql.AppendHelmLogsParams, len(lines))
	for i, l := range lines {
		params[i] = fasitdsql.AppendHelmLogsParams{
			Diid:    diid,
			Time:    time.Unix(0, l.GetTimeUnixNano()),
			Message: l.GetMsg(),
			Kind:    l.GetKind(),
		}
	}
	br := s.querier.AppendHelmLogs(ctx, params)
	var outerErr error
	br.Exec(func(_ int, err error) {
		if err != nil {
			outerErr = multierror.Append(outerErr, err)
		}
	})
	return outerErr
}

func (s *Server) setReleases(ctx context.Context, envID uuid.UUID, inv *protogen.ReleaseInventory) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin release tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := fasitdsql.New(tx)
	if err := q.DeleteReleaseStatusesInEnvironment(ctx, envID); err != nil {
		return fmt.Errorf("delete release statuses: %w", err)
	}
	for _, r := range inv.GetReleases() {
		if err := q.SetReleaseStatus(ctx, fasitdsql.SetReleaseStatusParams{
			EnvironmentID: envID,
			Feature:       r.GetName(),
			Version:       r.GetVersion(),
			Status:        r.GetStatus(),
			Revision:      r.GetRevision(),
			LastDeployed:  time.Unix(r.GetLastDeployedUnix(), 0),
		}); err != nil {
			return fmt.Errorf("set release status: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit release tx: %w", err)
	}
	return nil
}

func (s *Server) countReport(ctx context.Context, kind string) {
	if s.reportsRecv != nil {
		s.reportsRecv.Add(ctx, 1, metric.WithAttributes(attribute.String("type", kind)))
	}
}
