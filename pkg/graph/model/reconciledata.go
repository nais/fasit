package model

import "github.com/google/uuid"

type TenantEnvironments struct {
	Environment
	TenantName string
	TenantID   uuid.UUID
}
