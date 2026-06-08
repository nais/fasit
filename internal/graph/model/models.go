package model

import (
	"encoding/json"

	"github.com/google/uuid"
)

type HelmValueDiff struct {
	Difference HelmValueDifference `json:"difference"`
	Diff       string              `json:"diff"`
}

type RolloutLog struct {
	ID          uuid.UUID  `json:"id"`
	TenantName  string     `json:"tenantName"`
	Environment string     `json:"environment"`
	Lines       []*LogLine `json:"lines"`
}

type UpdateConfiguration struct {
	Description *string         `json:"description,omitempty"`
	Value       json.RawMessage `json:"value"`
}

type ConfigSource string

const (
	ConfigSourceGlobal  ConfigSource = "GLOBAL"
	ConfigSourceEnv     ConfigSource = "ENV"
	ConfigSourceHelm    ConfigSource = "HELM"
	ConfigSourceUnknown ConfigSource = "UNKNOWN"
)

func (e ConfigSource) IsValid() bool {
	switch e {
	case ConfigSourceGlobal, ConfigSourceEnv, ConfigSourceHelm, ConfigSourceUnknown:
		return true
	}
	return false
}

func (e ConfigSource) String() string {
	return string(e)
}

type HelmValueDifference string

const (
	HelmValueDifferenceFullMatch     HelmValueDifference = "FULL_MATCH"
	HelmValueDifferenceSupersetMatch HelmValueDifference = "SUPERSET_MATCH"
	HelmValueDifferenceNoMatch       HelmValueDifference = "NO_MATCH"
	HelmValueDifferenceInvalidJSON   HelmValueDifference = "INVALID_JSON"
)
