package model

import (
	"time"

	"github.com/google/uuid"
)

type DeployStatus string

const (
	DeployStatusUnknown    DeployStatus = ""
	DeployStatusSent       DeployStatus = "sent"
	DeployStatusInstalling DeployStatus = "installing"
	DeployStatusDeployed   DeployStatus = "deployed"
	DeployStatusFailed     DeployStatus = "failed"
)

func (r DeployStatus) IsValid() bool {
	switch r {
	case DeployStatusUnknown, DeployStatusSent, DeployStatusInstalling, DeployStatusDeployed, DeployStatusFailed:
		return true
	}
	return false
}

// IsInProgress reports whether the rollout has been dispatched but has not yet
// reached a terminal state (deployed/failed).
func (r DeployStatus) IsInProgress() bool {
	return r == DeployStatusSent || r == DeployStatusInstalling
}

func (r DeployStatus) String() string {
	return string(r)
}

type LogLine struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`

	IntID               int       `json:"-"`
	DeployInstructionID uuid.UUID `json:"-"`
}
