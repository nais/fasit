package model

import (
	"time"

	"github.com/google/uuid"
)

type Rollout struct {
	ID      uuid.UUID `json:"id"`
	Version string    `json:"version"`
	Chart   string    `json:"chart"`
	Created time.Time `json:"created"`

	FeatureName string `json:"-"`

	GraphVars struct {
		EnvironmentID uuid.UUID
	} `json:"-"`
}
