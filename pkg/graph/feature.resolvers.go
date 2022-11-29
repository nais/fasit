package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"sort"

	"github.com/nais/fasit/pkg/graph/graphgen"
	"github.com/nais/fasit/pkg/graph/model"
)

// Environment is the resolver for the environment field.
func (r *configOverrideResolver) Environment(ctx context.Context, obj *model.ConfigOverride) (*model.Environment, error) {
	return r.Repo.EnvironmentGet(ctx, obj.EnvironmentID)
}

// RolloutSummaries is the resolver for the rolloutSummaries field.
func (r *featureResolver) RolloutSummaries(ctx context.Context, obj *model.Feature) ([]*model.RolloutSummary, error) {
	return r.Repo.RolloutSummariesByFeature(ctx, obj.Name)
}

// Configoverrides is the resolver for the configoverrides field.
func (r *featureResolver) Configoverrides(ctx context.Context, obj *model.Feature) ([]*model.ConfigOverride, error) {
	return r.Repo.ConfigOverridesByFeature(ctx, obj.Name)
}

// OutdatedInfo is the resolver for the outdatedInfo field.
func (r *featureResolver) OutdatedInfo(ctx context.Context, obj *model.Feature) ([]*model.OutdatedInfo, error) {
	version := r.HelmChartValues.GetVersion(obj.Name)

	outdated := makeOutdatedInfo(obj.Name, version)

	sort.Slice(outdated, func(i, j int) bool {
		return outdated[i].FeatureName < outdated[j].FeatureName
	})
	return outdated, nil
}

// Feature is the resolver for the feature field.
func (r *outdatedInfoResolver) Feature(ctx context.Context, obj *model.OutdatedInfo) (*model.Feature, error) {
	return r.resolveFeatureByName(obj.FeatureName)
}

// Features is the resolver for the features field.
func (r *queryResolver) Features(ctx context.Context, kind *model.EnvironmentKind) ([]*model.Feature, error) {
	features := []*model.Feature{}
	for _, feature := range r.Resolver.Features.Features() {
		if kind != nil && !contains(feature.EnvironmentKinds, *kind) {
			continue
		}

		tmp, err := marshalFeature(feature)
		if err != nil {
			return nil, err
		}
		features = append(features, tmp)
	}
	return features, nil
}

// Feature is the resolver for the feature field.
func (r *queryResolver) Feature(ctx context.Context, name string) (*model.Feature, error) {
	return r.resolveFeatureByName(name)
}

// OutdatedInfo is the resolver for the outdatedInfo field.
func (r *queryResolver) OutdatedInfo(ctx context.Context) ([]*model.OutdatedInfo, error) {
	versions := r.HelmChartValues.AllVersions()
	ret := []*model.OutdatedInfo{}
	for name, version := range versions {
		ret = append(ret, makeOutdatedInfo(name, version)...)
	}

	return ret, nil
}

// ConfigOverride returns graphgen.ConfigOverrideResolver implementation.
func (r *Resolver) ConfigOverride() graphgen.ConfigOverrideResolver {
	return &configOverrideResolver{r}
}

// Feature returns graphgen.FeatureResolver implementation.
func (r *Resolver) Feature() graphgen.FeatureResolver { return &featureResolver{r} }

// OutdatedInfo returns graphgen.OutdatedInfoResolver implementation.
func (r *Resolver) OutdatedInfo() graphgen.OutdatedInfoResolver { return &outdatedInfoResolver{r} }

type configOverrideResolver struct{ *Resolver }
type featureResolver struct{ *Resolver }
type outdatedInfoResolver struct{ *Resolver }
