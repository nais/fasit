package environment

import (
	"github.com/nais/fasit/internal/environment/environmentsql"
	"github.com/nais/fasit/internal/graph/model"
)

type Labels map[string]string

func environmentFromSQL(e environmentsql.Environment) *model.Environment {
	return &model.Environment{
		ID:           e.ID,
		Name:         e.Name,
		Description:  e.Description,
		Created:      e.Created.Time,
		LastModified: e.LastModified.Time,
		Kind:         model.EnvironmentKind(e.Kind),
		TenantID:     e.TenantID,
		Reconcile:    e.Reconcile,
		Labels:       e.Labels,
	}
}

func tenantFromSQL(t environmentsql.Tenant) *model.Tenant {
	return &model.Tenant{
		ID:           t.ID,
		Name:         t.Name,
		Description:  t.Description,
		Created:      t.Created.Time,
		LastModified: t.LastModified.Time,
	}
}
