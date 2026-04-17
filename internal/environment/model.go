package environment

import (
	"github.com/nais/fasit/internal/environment/environmentsql"
	"github.com/nais/fasit/internal/graph/model"
)

type Labels map[string]string

// TODO: add labels to the model below. It currently exists as a GraphQL resolver.
func environmentFromSQL(p environmentsql.Environment) *model.Environment {
	return &model.Environment{
		ID:           p.ID,
		Name:         p.Name,
		Description:  p.Description,
		Created:      p.Created.Time,
		LastModified: p.LastModified.Time,
		Kind:         model.EnvironmentKind(p.Kind),
		TenantID:     p.TenantID,
		CI:           p.Ci,
		Reconcile:    p.Reconcile,
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
