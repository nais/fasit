package model

import (
	"time"

	"github.com/google/uuid"
)

type Cost struct {
	Date time.Time `json:"date"`
	Cost float64   `json:"cost"`

	TenantID uuid.UUID `json:"tenantId"`
	EnvID    uuid.UUID `json:"envId"`
}
