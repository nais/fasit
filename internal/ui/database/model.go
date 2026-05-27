package database

import (
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/ui/database/sqlgen"
)

type Tenant struct {
	ID           uuid.UUID
	Name         string
	Description  *string
	Created      time.Time
	LastModified time.Time
}

/*
func environmentFromSQL(e sqlgen.Environment) *model.Environment {
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

*/

func tenantFromSQL(t sqlgen.Tenant) *Tenant {
	return &Tenant{
		ID:           t.ID,
		Name:         t.Name,
		Description:  t.Description,
		Created:      t.Created.Time,
		LastModified: t.LastModified.Time,
	}
}
