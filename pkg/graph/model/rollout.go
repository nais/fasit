package model

import (
	"time"

	"github.com/google/uuid"
)

type Rollout struct {
	ID        uuid.UUID     `json:"id"`
	Version   string        `json:"version"`
	Created   time.Time     `json:"created"`
	Completed *time.Time    `json:"completed"`
	Status    RolloutStatus `json:"status"`

	FeatureName string `json:"-"`

	GraphVars struct {
		EnvironmentID uuid.UUID
	} `json:"-"`
}
