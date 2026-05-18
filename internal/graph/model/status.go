package model

import (
	"time"

	"github.com/google/uuid"
)

type RolloutStatus string

const (
	RolloutStatusUnknown  RolloutStatus = ""
	RolloutStatusCreated  RolloutStatus = "created"
	RolloutStatusPending  RolloutStatus = "pending"
	RolloutStatusDeployed RolloutStatus = "deployed"
	RolloutStatusFailed   RolloutStatus = "failed"
)

func (r RolloutStatus) IsValid() bool {
	switch r {
	case RolloutStatusUnknown, RolloutStatusCreated, RolloutStatusPending, RolloutStatusDeployed, RolloutStatusFailed:
		return true
	}
	return false
}

func (r RolloutStatus) String() string {
	return string(r)
}

type LogLine struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`

	IntID               int       `json:"-"`
	DeployInstructionID uuid.UUID `json:"-"`
}
