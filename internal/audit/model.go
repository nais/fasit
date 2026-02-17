package audit

import (
	"github.com/nais/fasit/internal/audit/auditsql"
	"github.com/nais/fasit/internal/graph/model"
)

func auditLogsFromSQL(auditLogs []auditsql.Audit) []*model.AuditLog {
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
