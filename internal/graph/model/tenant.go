package model

import (
	"time"

	"github.com/google/uuid"
)

type Tenant struct {
	ID               uuid.UUID `json:"id"`
	Name             string    `json:"name"`
	Description      *string   `json:"description"`
	Created          time.Time `json:"created"`
	LastModified     time.Time `json:"lastModified"`
	UpgradeDelayDays int32     `json:"upgradeDelayDays"`
}
