package uidata

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/featureassignment"
)

func ListTenants(ctx context.Context) ([]*Tenant, error) {
	rows, err := querier(ctx).ListTenants(ctx)
	if err != nil {
		return nil, err
	}
	ret := make([]*Tenant, len(rows))
	for i, tenant := range rows {
		ret[i] = tenantFromSQL(tenant)
	}
	return ret, nil
}

func ListEnvironmentFeatures(ctx context.Context, environmentID uuid.UUID) ([]EnvironmentFeature, error) {
	assignments, err := featureassignment.ListForEnvironment(ctx, environmentID)
	if err != nil {
		return nil, err
	}

	features := make([]EnvironmentFeature, 0)
	for _, a := range assignments {
		features = append(features, EnvironmentFeature{
			Name:            a.Feature.Name,
			FeatureDisabled: a.FeatureDisabled,
		})
	}
	sort.Slice(features, func(i, j int) bool { return features[i].Name < features[j].Name })
	return features, nil
}

func ListDeployInstructions(ctx context.Context, featureAssignmentID uuid.UUID) ([]*DeployInstruction, error) {
	rows, err := querier(ctx).ListDeployInstructions(ctx, &featureAssignmentID)
	if err != nil {
		return nil, err
	}
	ret := make([]*DeployInstruction, len(rows))
	for i, row := range rows {
		ret[i] = &DeployInstruction{
			ID:                  row.ID,
			EnvironmentID:       row.EnvironmentID,
			FeatureAssignmentID: row.FeatureAssignmentID,
			FeatureName:         row.FeatureName,
			FeatureVersion:      row.FeatureVersion,
			Status:              row.Status,
			Hash:                row.Hash,
			Created:             row.Created.Time,
			LastModified:        row.LastModified.Time,
			Values:              row.Values,
			TenantName:          row.TenantName,
			EnvironmentName:     row.EnvironmentName,
		}
	}

	return ret, nil
}

func GetEnvironmentValueReferences(ctx context.Context, envID uuid.UUID) (EnvironmentValueReferences, error) {
	assignments, err := featureassignment.ListForEnvironment(ctx, envID)
	if err != nil {
		return nil, fmt.Errorf("list feature assignments for environment: %w", err)
	}

	// key → set of feature names
	refSets := make(map[string]map[string]bool)
	for _, dep := range assignments {
		if len(dep.Feature.TplDetails) == 0 {
			continue
		}
		var details struct {
			Env        []string `json:"Env"`
			Envs       []string `json:"Envs"`
			Management []string `json:"Management"`
		}
		if err := json.Unmarshal(dep.Feature.TplDetails, &details); err != nil {
			slog.With("err", err, "feature", dep.Feature.Name).Warn("failed to unmarshal tpl_details")
			continue
		}
		seen := make(map[string]bool)
		for _, key := range details.Env {
			seen[key] = true
		}
		for _, key := range details.Envs {
			seen[key] = true
		}
		for _, key := range details.Management {
			seen[key] = true
		}
		for key := range seen {
			if refSets[key] == nil {
				refSets[key] = make(map[string]bool)
			}
			refSets[key][dep.Feature.Name] = true
		}
	}
	refs := make(EnvironmentValueReferences, len(refSets))
	for key, names := range refSets {
		for name := range names {
			refs[key] = append(refs[key], name)
		}
	}
	return refs, nil
}
