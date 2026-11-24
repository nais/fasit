package fasitd

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/nais/fasit/internal/fasitd/commandstatus"
	"github.com/nais/fasit/internal/fasitd/fasitdsql"
	"github.com/nais/fasit/internal/fasitd/protogen"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func (s *Server) markUndeliverable(ctx context.Context, msg *protogen.ServerMessage, reason string) {
	diid := msg.GetCommand().GetDiid()
	if err := s.appendStatus(ctx, diid, string(commandstatus.Undeliverable), reason); err != nil {
		s.log.With("err", err, "diid", diid).Error("mark command undeliverable")
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

func (s *Server) countReport(ctx context.Context, kind MessageKind) {
	if s.reportsRecv != nil {
		s.reportsRecv.Add(ctx, 1, metric.WithAttributes(attribute.String("type", string(kind))))
	}
}
