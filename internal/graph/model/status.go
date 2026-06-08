package model

import (
	"time"

	"github.com/google/uuid"
)

type RolloutStatus string

const (
	RolloutStatusUnknown    RolloutStatus = ""
	RolloutStatusSent       RolloutStatus = "sent"
	RolloutStatusInstalling RolloutStatus = "installing"
	RolloutStatusDeployed   RolloutStatus = "deployed"
	RolloutStatusFailed     RolloutStatus = "failed"
)

func (r RolloutStatus) IsValid() bool {
	switch r {
	case RolloutStatusUnknown, RolloutStatusSent, RolloutStatusInstalling, RolloutStatusDeployed, RolloutStatusFailed:
		return true
	}
	return false
}

// IsInProgress reports whether the rollout has been dispatched but has not yet
// reached a terminal state (deployed/failed).
func (r RolloutStatus) IsInProgress() bool {
	return r == RolloutStatusSent || r == RolloutStatusInstalling
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
