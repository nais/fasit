package model

import "github.com/google/uuid"

type DeployInstruction struct {
	ID             uuid.UUID
	EnvironmentID  uuid.UUID
	FeatureName    string
	FeatureVersion string
	Hash           string
}
