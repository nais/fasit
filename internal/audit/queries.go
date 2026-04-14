package audit

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/audit/auditsql"
	"github.com/nais/fasit/internal/auth"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/sirupsen/logrus"
)

type ctxKey int

const managerKey ctxKey = iota

type manager struct {
	querier auditsql.Querier
	log     logrus.FieldLogger
	// auditErrorCount metric.Int64Counter
}

func Register(ctx context.Context, pool *pgxpool.Pool, log logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, managerKey, &manager{querier: auditsql.New(pool), log: log})
}

func querier(ctx context.Context) auditsql.Querier {
	return ctx.Value(managerKey).(*manager).querier
}

func log(ctx context.Context) logrus.FieldLogger {
	return ctx.Value(managerKey).(*manager).log
}

func CreateAudit(ctx context.Context, description, objectType, objectID string) {
	actor := auth.GetEmail(ctx)
	if actor == auth.UnauthorizedName {
		log(ctx).WithFields(logrus.Fields{
			"description": description,
			"objectType":  objectType,
		}).Warn("unknown actor")
		actor = "unknown"
	}

	err := querier(ctx).AuditCreate(ctx, auditsql.AuditCreateParams{
		Actor:       actor,
		Description: description,
		ObjectType:  objectType,
		ObjectID:    objectID,
	})
	if err != nil {
		log(ctx).WithError(err).Error("failed to create audit")
		// r.auditErrorCount.Add(ctx, 1)
	}
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
