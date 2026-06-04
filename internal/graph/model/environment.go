package model

import (
	"time"

	"github.com/google/uuid"
)

type Environment struct {
	ID           uuid.UUID       `json:"id"`
	Name         string          `json:"name"`
	Description  *string         `json:"description"`
	Created      time.Time       `json:"created"`
	LastModified time.Time       `json:"lastModified"`
	Kind         EnvironmentKind `json:"kind"`
	Reconcile    bool            `json:"reconciled"`

	TenantID uuid.UUID         `json:"tenantID"`
	Labels   map[string]string `json:"labels"`
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
	switch e {
	case EnvironmentKindTenant, EnvironmentKindManagement, EnvironmentKindOnprem:
		return true
	}
	return false
}

func (e EnvironmentKind) String() string {
	return string(e)
}
