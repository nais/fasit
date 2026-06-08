package environment

import (
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/environment/environmentsql"
)

type Labels map[string]string

type Environment struct {
	ID           uuid.UUID         `json:"id"`
	Name         string            `json:"name"`
	Description  *string           `json:"description"`
	Created      time.Time         `json:"created"`
	LastModified time.Time         `json:"lastModified"`
	Kind         EnvironmentKind   `json:"kind"`
	Reconcile    bool              `json:"reconciled"`
	TenantID     uuid.UUID         `json:"tenantID"`
	Labels       map[string]string `json:"labels"`
}

type EnvironmentKind string

const (
	EnvironmentKindTenant     EnvironmentKind = "tenant"
	EnvironmentKindManagement EnvironmentKind = "management"
	EnvironmentKindOnprem     EnvironmentKind = "onprem"
)

var AllEnvironmentKind = []EnvironmentKind{
	EnvironmentKindTenant,
	EnvironmentKindManagement,
	EnvironmentKindOnprem,
}

func (e EnvironmentKind) IsValid() bool {
	return slices.Contains(AllEnvironmentKind, e)
}

func (e EnvironmentKind) String() string {
	return string(e)
}

type EnvironmentCreate struct {
	Name        string            `json:"name"`
	Description *string           `json:"description,omitempty"`
	TenantID    uuid.UUID         `json:"tenantID"`
	Kind        EnvironmentKind   `json:"kind"`
	Labels      map[string]string `json:"labels,omitempty"`
}

type EnvironmentLabel struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func environmentFromSQL(e environmentsql.Environment) *Environment {
	return &Environment{
		ID:           e.ID,
		Name:         e.Name,
		Description:  e.Description,
		Created:      e.Created,
		LastModified: e.LastModified,
		Kind:         EnvironmentKind(e.Kind),
		TenantID:     e.TenantID,
		Reconcile:    e.Reconcile,
		Labels:       e.Labels,
	}
}
