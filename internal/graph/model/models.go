package model

import (
	"github.com/google/uuid"
)

type RolloutLog struct {
	ID          uuid.UUID  `json:"id"`
	TenantName  string     `json:"tenantName"`
	Environment string     `json:"environment"`
	Lines       []*LogLine `json:"lines"`
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
