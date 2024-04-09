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
	GHRef     *GHRef        `json:"ghRef"`

	FeatureName string `json:"-"`

	GraphVars struct {
		EnvironmentID      uuid.UUID
		DeployInstructions []uuid.UUID
	} `json:"-"`
}

type GHRef struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Ref   string `json:"ref"`
}
