package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/graph/graphgen"
	"github.com/nais/fasit/pkg/graph/model"
)

// FeatureStates is the resolver for the featureStates field.
func (r *environmentResolver) FeatureStates(ctx context.Context, obj *model.Environment) ([]*model.FeatureState, error) {
	retVal, err := r.Repo.FeatureStatesGet(ctx, obj.ID)
	if err != nil {
		return nil, err
	}

OUTER:
	for _, f := range r.Features.Features {
		if !contains(f.EnvironmentKinds, obj.Kind) {
			continue
		}

		// Skip elements that are configured
		for _, c := range retVal {
			if f.Name == c.FeatureName {
				continue OUTER
			}
		}
		retVal = append(retVal, &model.FeatureState{FeatureName: f.Name})
	}

	sort.Slice(retVal, func(i, j int) bool {
		return retVal[i].FeatureName < retVal[j].FeatureName
	})

	return retVal, nil
}

// Health is the resolver for the health field.
func (r *environmentResolver) Health(ctx context.Context, obj *model.Environment) (*model.Health, error) {
	health, err := r.Repo.HealthGet(ctx, obj.ID)
	if err != nil {
		return &model.Health{
			EnvironmentID: obj.ID,
			ReportedAt:    time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		}, nil
	}
	return health, nil
}

// Releases is the resolver for the releases field.
func (r *environmentResolver) Releases(ctx context.Context, obj *model.Environment) ([]*model.Release, error) {
	return r.Repo.ReleaseStatusesGet(ctx, obj.ID)
}

// Nodes is the resolver for the nodes field.
func (r *environmentResolver) Nodes(ctx context.Context, obj *model.Environment) ([]*model.KubernetesNode, error) {
	return r.Repo.KubernetesNodesForEnv(ctx, obj.ID)
}

// Values is the resolver for the values field.
func (r *environmentResolver) Values(ctx context.Context, obj *model.Environment) ([]*model.EnvironmentValue, error) {
	return r.Repo.EnvironmentValuesForEnvironment(ctx, obj.ID, false)
}

// EnvironmentCreate is the resolver for the environmentCreate field.
func (r *mutationResolver) EnvironmentCreate(ctx context.Context, environment model.EnvironmentCreate) (*model.Environment, error) {
	return r.Repo.EnvironmentCreate(ctx, &environment)
}

// EnvironmentUpdate is the resolver for the environmentUpdate field.
func (r *mutationResolver) EnvironmentUpdate(ctx context.Context, id uuid.UUID, input model.EnvironmentUpdate) (*model.Environment, error) {
	return r.Repo.EnvironmentUpdate(ctx, id, &input)
}

// Environment is the resolver for the environment field.
func (r *queryResolver) Environment(ctx context.Context, id uuid.UUID) (*model.Environment, error) {
	return r.Repo.EnvironmentGet(ctx, id)
}

// EnvironmentByNames is the resolver for the environmentByNames field.
func (r *queryResolver) EnvironmentByNames(ctx context.Context, environmentName string, tenantName string) (*model.Environment, error) {
	return r.Repo.EnvironmentByNames(ctx, tenantName, environmentName)
}

// Environments is the resolver for the environments field.
func (r *queryResolver) Environments(ctx context.Context, tenantID uuid.UUID) ([]*model.Environment, error) {
	return r.Repo.EnvironmentsGet(ctx, tenantID)
}

// Feature is the resolver for the feature field.
func (r *releaseResolver) Feature(ctx context.Context, obj *model.Release) (*model.Feature, error) {
	return r.resolveFeatureByName(obj.FeatureName)
}

// Environment returns graphgen.EnvironmentResolver implementation.
func (r *Resolver) Environment() graphgen.EnvironmentResolver { return &environmentResolver{r} }

// Release returns graphgen.ReleaseResolver implementation.
func (r *Resolver) Release() graphgen.ReleaseResolver { return &releaseResolver{r} }

type environmentResolver struct{ *Resolver }
type releaseResolver struct{ *Resolver }
