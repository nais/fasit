package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/graph/graphgen"
	"github.com/nais/fasit/pkg/graph/model"
)

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

	return retVal, nil
}

func (r *environmentResolver) Health(ctx context.Context, obj *model.Environment) (*model.Health, error) {
	return r.Repo.HealthGet(ctx, obj.ID)
}

func (r *environmentResolver) Releases(ctx context.Context, obj *model.Environment) ([]*model.Release, error) {
	return r.Repo.ReleaseStatusesGet(ctx, obj.ID)
}

func (r *mutationResolver) EnvironmentCreate(ctx context.Context, environment model.EnvironmentCreate) (*model.Environment, error) {
	return r.Repo.EnvironmentCreate(ctx, &environment)
}

func (r *mutationResolver) EnvironmentUpdate(ctx context.Context, id uuid.UUID, input model.EnvironmentUpdate) (*model.Environment, error) {
	return r.Repo.EnvironmentUpdate(ctx, id, &input)
}

func (r *queryResolver) Environment(ctx context.Context, id uuid.UUID) (*model.Environment, error) {
	return r.Repo.EnvironmentGet(ctx, id)
}

func (r *queryResolver) EnvironmentByNames(ctx context.Context, environmentName string, tenantName string) (*model.Environment, error) {
	return r.Repo.EnvironmentByNames(ctx, tenantName, environmentName)
}

func (r *queryResolver) Environments(ctx context.Context, tenantID uuid.UUID) ([]*model.Environment, error) {
	return r.Repo.EnvironmentsGet(ctx, tenantID)
}

func (r *releaseResolver) Feature(ctx context.Context, obj *model.Release) (*model.Feature, error) {
	return r.resolveFeatureByName(obj.FeatureName)
}

// Environment returns graphgen.EnvironmentResolver implementation.
func (r *Resolver) Environment() graphgen.EnvironmentResolver { return &environmentResolver{r} }

// Release returns graphgen.ReleaseResolver implementation.
func (r *Resolver) Release() graphgen.ReleaseResolver { return &releaseResolver{r} }

type environmentResolver struct{ *Resolver }
type releaseResolver struct{ *Resolver }
