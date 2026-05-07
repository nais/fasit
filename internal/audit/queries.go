package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/audit/auditsql"
	"github.com/nais/fasit/internal/auth"
	"github.com/nais/fasit/internal/dbtx"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/sirupsen/logrus"
)

type ctxKey int

// QuerierKey is exposed so tests can inject mock queriers on the context.
const (
	QuerierKey ctxKey = iota
	logKey
)

func Register(ctx context.Context, pool *pgxpool.Pool, log logrus.FieldLogger) context.Context {
	ctx = context.WithValue(ctx, QuerierKey, auditsql.Querier(auditsql.New(pool)))
	ctx = context.WithValue(ctx, logKey, log)
	return ctx
}

func RegisterTestDeps(ctx context.Context, q auditsql.Querier, log logrus.FieldLogger) context.Context {
	ctx = context.WithValue(ctx, QuerierKey, q)
	ctx = context.WithValue(ctx, logKey, log)
	return ctx
}

func querier(ctx context.Context) auditsql.Querier {
	q := ctx.Value(QuerierKey).(auditsql.Querier)
	if tx, ok := dbtx.Tx(ctx); ok {
		if real, ok := q.(*auditsql.Queries); ok {
			return real.WithTx(tx)
		}
	}
	return q
}

func log(ctx context.Context) logrus.FieldLogger {
	return ctx.Value(logKey).(logrus.FieldLogger)
}

func CreateAudit(ctx context.Context, description, objectType, objectID string) {
	actor := actorOrUnknown(ctx, description, objectType)

	err := querier(ctx).AuditCreate(ctx, auditsql.AuditCreateParams{
		Actor:       actor,
		Description: description,
		ObjectType:  objectType,
		ObjectID:    objectID,
	})
	if err != nil {
		log(ctx).WithError(err).Error("failed to create audit")
	}
}

type CreateParams struct {
	Description string
	ObjectType  string
	ObjectID    string
	// Metadata is JSON-marshaled before insert. nil → SQL NULL.
	Metadata any
}

// Create writes an audit row, returning any error so callers can roll the
// surrounding transaction back.
func Create(ctx context.Context, p CreateParams) error {
	actor := actorOrUnknown(ctx, p.Description, p.ObjectType)

	var meta []byte
	if p.Metadata != nil {
		b, err := json.Marshal(p.Metadata)
		if err != nil {
			return fmt.Errorf("marshal audit metadata: %w", err)
		}
		meta = b
	}

	return querier(ctx).AuditCreate(ctx, auditsql.AuditCreateParams{
		Actor:       actor,
		Description: p.Description,
		ObjectType:  p.ObjectType,
		ObjectID:    p.ObjectID,
		Metadata:    meta,
	})
}

func actorOrUnknown(ctx context.Context, description, objectType string) string {
	actor := auth.GetEmail(ctx)
	if actor == auth.UnauthorizedName {
		log(ctx).WithFields(logrus.Fields{
			"description": description,
			"objectType":  objectType,
		}).Warn("unknown actor")
		return "unknown"
	}
	return actor
}

func AuditForEnvironment(ctx context.Context, id uuid.UUID, featureName string) ([]*model.AuditLog, error) {
	auditLogs, err := querier(ctx).AuditForEnvironment(ctx, auditsql.AuditForEnvironmentParams{
		EnvironmentID: id.String(),
		Featurename:   featureName,
		PageSize:      50,
	})
	if err != nil {
		return nil, err
	}

	return auditLogsFromSQL(auditLogs), nil
}

func AuditDeleteHelmInstall(ctx context.Context, envID uuid.UUID, name string) {
	CreateAudit(ctx, "delete helm install "+name, "environments", envID.String())
}
