package environment

import (
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/environment/environmentsql"
)

type Tenant struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Description  *string   `json:"description"`
	Created      time.Time `json:"created"`
	LastModified time.Time `json:"lastModified"`
}

type TenantEnvironment struct {
	Environment
	TenantName string
	TenantID   uuid.UUID
}

type TenantCreate struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

func tenantFromSQL(t environmentsql.Tenant) *Tenant {
	return &Tenant{
		ID:           t.ID,
		Name:         t.Name,
		Description:  t.Description,
		Created:      t.Created,
		LastModified: t.LastModified,
	}
}
