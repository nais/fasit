package model

import "github.com/google/uuid"

type TenantEnvironment struct {
	Environment
	TenantName string
	TenantID   uuid.UUID
}
