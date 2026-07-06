package uidata

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database/types"
	"github.com/nais/fasit/internal/ui/uidata/sqlgen"
)

type EnvironmentValueReferences = map[string][]string

type Environment struct {
	ID           uuid.UUID
	Name         string
	Description  *string
	Created      time.Time
	LastModified time.Time
	Kind         types.EnvironmentKind
	Reconcile    bool
	Labels       types.EnvironmentLabels
}

type EnvironmentFeature struct {
	Name            string
	FeatureDisabled bool
}

type Tenant struct {
	ID           uuid.UUID
	Name         string
	Description  *string
	Created      time.Time
	LastModified time.Time

	envs []*Environment
}

type DeployInstruction struct {
	ID                  uuid.UUID
	EnvironmentID       uuid.UUID
	FeatureName         string
	FeatureVersion      string
	Status              string
	Hash                string
	Created             time.Time
	LastModified        time.Time
	FeatureAssignmentID uuid.UUID
	EnvironmentName     string
	TenantName          string
}

func (t *Tenant) Environments(ctx context.Context) ([]*Environment, error) {
	if len(t.envs) > 0 {
		return t.envs, nil
	}

	rows, err := querier(ctx).ListTenantEnvironments(ctx, t.ID)
	if err != nil {
		return nil, err
	}

	t.envs = make([]*Environment, len(rows))
	for i, row := range rows {
		t.envs[i] = environmentFromSQL(row)
	}

	return t.envs, nil
}

func environmentFromSQL(e sqlgen.Environment) *Environment {
	return &Environment{
		ID:           e.ID,
		Name:         e.Name,
		Description:  e.Description,
		Created:      e.Created,
		LastModified: e.LastModified,
		Kind:         e.Kind,
		Reconcile:    e.Reconcile,
		Labels:       e.Labels,
	}
}

func tenantFromSQL(t sqlgen.Tenant) *Tenant {
	return &Tenant{
		ID:           t.ID,
		Name:         t.Name,
		Description:  t.Description,
		Created:      t.Created,
		LastModified: t.LastModified,
	}
}

func logLineFromSQL(log sqlgen.Log) *LogLine {
	return &LogLine{
		ID:                  fmt.Sprintf("%s-%d", log.DeployInstruction, log.ID),
		Timestamp:           log.Time,
		Message:             log.Message,
		IntID:               int(log.ID),
		DeployInstructionID: log.DeployInstruction,
	}
}

type LogLine struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`

	IntID               int       `json:"-"`
	DeployInstructionID uuid.UUID `json:"-"`
}

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

type FeatureVersion struct {
	Version     string
	Description string
	Source      string
	LastUpdated time.Time
}
