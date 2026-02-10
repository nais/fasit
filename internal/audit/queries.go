package audit

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/audit/auditsql"
	"github.com/nais/fasit/internal/auth"
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
