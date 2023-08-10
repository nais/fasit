package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/auth"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/sirupsen/logrus"
)

type AuditRepo interface {
	AuditForEnvironment(ctx context.Context, id uuid.UUID, featureName string) ([]*model.AuditLog, error)
}

// AuditForEnvironment returns all audit logs for a given environment. If featureName is provided, only logs for that feature are returned.
func (r *repo) AuditForEnvironment(ctx context.Context, id uuid.UUID, featureName string) ([]*model.AuditLog, error) {
	auditLogs, err := r.querier.AuditForEnvironment(ctx, gensql.AuditForEnvironmentParams{
		EnvironmentID: id.String(),
		Featurename:   featureName,
		PageSize:      50,
	})
	if err != nil {
		return nil, err
	}

	return auditLogsFromSQL(auditLogs), nil
}

func (r *repo) createAudit(ctx context.Context, description, objectType, objectID string) {
	actor := auth.GetEmail(ctx)
	if actor == auth.UnauthorizedName {
		r.log.WithFields(logrus.Fields{
			"description": description,
			"objectType":  objectType,
		}).Warn("unknown actor")
		actor = "unknown"
	}

	err := r.querier.AuditCreate(ctx, gensql.AuditCreateParams{
		Actor:       actor,
		Description: description,
		ObjectType:  objectType,
		ObjectID:    objectID,
	})
	if err != nil {
		r.log.WithError(err).Error("failed to create audit")
		r.auditErrorCount.Add(ctx, 1)
	}
}

func auditLogsFromSQL(auditLogs []gensql.Audit) []*model.AuditLog {
	var result []*model.AuditLog
	for _, auditLog := range auditLogs {
		result = append(result, &model.AuditLog{
			Actor:       auditLog.Actor,
			Description: auditLog.Description,
			ObjectType:  auditLog.ObjectType,
			ObjectID:    auditLog.ObjectID,
			CreatedAt:   auditLog.CreatedAt.Time,
		})
	}
	return result
}
