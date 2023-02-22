package model

import (
	"encoding/json"

	"github.com/google/uuid"
)

type Feature struct {
	Name             string            `json:"name"`
	Chart            string            `json:"chart"`
	Version          string            `json:"version"`
	Description      string            `json:"description"`
	Source           string            `json:"source"`
	Dependencies     []*Dependency     `json:"dependsOn"`
	Config           json.RawMessage   `json:"config"`
	EnvironmentKinds []EnvironmentKind `json:"environmentKinds"`
}

type ConfigOverride struct {
	EnvironmentID uuid.UUID
	Keys          []string `json:"keys"`
}

type OutdatedInfo struct {
	Dependency     bool   `json:"dependency"`
	DependencyName string `json:"dependencyName"`
	NewVersion     string `json:"newVersion"`

	FeatureName string `json:"-"`
}
