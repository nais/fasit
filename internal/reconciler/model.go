package reconciler

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/reconciler/reconcilersql"
)

type environment struct {
	ID         uuid.UUID
	Name       string
	Kind       model.EnvironmentKind
	Labels     map[string]string
	TenantID   uuid.UUID
	TenantName string
}

type reconcileDeployment struct {
	ID           uuid.UUID
	Feature      *model.Feature
	TargetLabels map[string]string
	Created      time.Time
}

type latestInstruction struct {
	Hash   string
	Status string
}

type renderAction int

const (
	actionDeploy renderAction = iota
	actionSkipUnchanged
	actionSkipInProgress
	actionSkipDisabled
	actionFailMissingDeps
	actionFailMissingConfig
	actionFailRender
)

type renderResult struct {
	EnvironmentID   uuid.UUID
	EnvironmentName string
	TenantName      string
	DeploymentID    uuid.UUID
	Feature         *model.Feature
	Values          map[string]any
	Hash            string
	Action          renderAction
	Message         string
	Status          string
}

func deploymentFromRow(row reconcilersql.ListLatestDeploymentsRow) (*reconcileDeployment, error) {
	var deps model.Dependencies
	if err := json.Unmarshal(row.Dependencies, &deps); err != nil {
		return nil, fmt.Errorf("unmarshal dependencies for %s: %w", row.FeatureName, err)
	}

	var values model.Values
	if err := json.Unmarshal(row.Values, &values); err != nil {
		return nil, fmt.Errorf("unmarshal values for %s: %w", row.FeatureName, err)
	}

	kinds := make([]model.EnvironmentKind, len(row.Kinds))
	for i, k := range row.Kinds {
		kinds[i] = model.EnvironmentKind(k)
	}

	var defaultValues map[string]json.RawMessage
	if err := json.Unmarshal(row.DefaultValues, &defaultValues); err != nil {
		return nil, fmt.Errorf("unmarshal default values for %s: %w", row.FeatureName, err)
	}

	return &reconcileDeployment{
		ID: row.ID,
		Feature: &model.Feature{
			FeatureYAML: model.FeatureYAML{
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
		},
		TargetLabels: map[string]string(row.Target),
		Created:      row.Created.Time,
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
