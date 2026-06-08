package reconciler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	environment2 "github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/reconciler/reconcilersql"
)

type environment struct {
	ID         uuid.UUID
	Name       string
	Kind       environment2.EnvironmentKind
	Labels     map[string]string
	TenantID   uuid.UUID
	TenantName string
}

type reconcileAssignment struct {
	ID           uuid.UUID
	Feature      *feature.Feature
	TargetLabels map[string]string
	Created      time.Time
}

type latestInstruction struct {
	Hash   string
	Status string
}

// Action describes the reconciler's decision for a single deployment×environment pair.
type Action string

const (
	ActionDeploy            Action = "deploy"
	ActionSkipUnchanged     Action = "unchanged"
	ActionSkipInProgress    Action = "in-progress"
	ActionSkipDisabled      Action = "disabled"
	ActionSkipUnhealthy     Action = "unhealthy"
	ActionFailMissingDeps   Action = "missing-deps"
	ActionFailMissingConfig Action = "missing-config"
	ActionFailRender        Action = "render-error"
)

func (a Action) String() string {
	return string(a)
}

func (a Action) IsFailure() bool {
	return a == ActionFailMissingDeps || a == ActionFailMissingConfig || a == ActionFailRender
}

// DeployDecision is the output of the compute phase for one deployment×environment pair.
type DeployDecision struct {
	EnvironmentID       uuid.UUID
	EnvironmentName     string
	TenantName          string
	FeatureAssignmentID uuid.UUID
	Feature             *feature.Feature
	Values              map[string]any
	Hash                string
	Action              Action
	Details             string
	Message             string
}

// ErrReconcileInProgress is returned when a streaming reconcile is already running.
var ErrReconcileInProgress = errors.New("reconcile already in progress")

// Deployer receives the full set of deploy decisions and performs
// side-effects (DB writes, message publishing, UI updates, etc.).
type Deployer interface {
	Deploy(ctx context.Context, decisions []DeployDecision) error
}

func assignmentFromRow(row reconcilersql.ListLatestFeatureAssignmentsRow) (*reconcileAssignment, error) {
	var deps feature.Dependencies
	if err := json.Unmarshal(row.Dependencies, &deps); err != nil {
		return nil, fmt.Errorf("unmarshal dependencies for %s: %w", row.FeatureName, err)
	}

	var values feature.Values
	if err := json.Unmarshal(row.Values, &values); err != nil {
		return nil, fmt.Errorf("unmarshal values for %s: %w", row.FeatureName, err)
	}

	kinds := make([]environment2.EnvironmentKind, len(row.Kinds))
	for i, k := range row.Kinds {
		kinds[i] = environment2.EnvironmentKind(k)
	}

	var defaultValues map[string]json.RawMessage
	if err := json.Unmarshal(row.DefaultValues, &defaultValues); err != nil {
		return nil, fmt.Errorf("unmarshal default values for %s: %w", row.FeatureName, err)
	}

	return &reconcileAssignment{
		ID: row.ID,
		Feature: &feature.Feature{
			FeatureYAML: feature.FeatureYAML{
				Dependencies:     deps,
				EnvironmentKinds: kinds,
				Timeout:          time.Duration(row.Timeout) * time.Millisecond,
				Values:           values,
			},
			Name:        row.FeatureName,
			Chart:       row.Chart,
			Version:     row.Version,
			Description: row.Description,
			Source:      row.Source,
			ValuesYAML:  defaultValues,
			SpecVersion: "v2",
			TplDetails:  row.TplDetails,
		},
		TargetLabels: row.Target,
		Created:      row.Created,
	}, nil
}

func labelsContain(envLabels, target map[string]string) bool {
	for k, v := range target {
		if envLabels[k] != v {
			return false
		}
	}
	return true
}
