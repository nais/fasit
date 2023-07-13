package model

import (
	"time"

	"github.com/google/uuid"
)

type DeployInstruction struct {
	ID             uuid.UUID     `json:"id"`
	EnvironmentID  uuid.UUID     `json:"environmentId"`
	FeatureName    string        `json:"-"`
	FeatureVersion string        `json:"-"`
	Status         RolloutStatus `json:"status"`
	Hash           string        `json:"-"`
	Created        time.Time     `json:"created"`
	LastModified   time.Time     `json:"lastModified"`
}
