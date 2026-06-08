package model

import (
	"time"

	"github.com/google/uuid"
)

type DeployInstruction struct {
	ID                  uuid.UUID
	EnvironmentID       uuid.UUID
	FeatureAssignmentID uuid.UUID
	FeatureName         string
	FeatureVersion      string
	Status              DeployStatus
	Hash                string
	Created             time.Time
	LastModified        time.Time
}
