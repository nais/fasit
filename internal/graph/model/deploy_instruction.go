package model

import (
	"time"

	"github.com/google/uuid"
)

type DeployInstruction struct {
	ID                  uuid.UUID
	EnvironmentID       uuid.UUID
	FeatureAssignmentID *uuid.UUID
	FeatureName         string
	FeatureVersion      string
	Status              RolloutStatus
	Hash                string
	Created             time.Time
	LastModified        time.Time

	// Helm values for this deploy instruction.
	Values []byte
}
