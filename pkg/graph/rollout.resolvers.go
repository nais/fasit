package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/graph/graphgen"
	"github.com/nais/fasit/pkg/graph/model"
)

// RolloutSummary is the resolver for the rolloutSummary field.
func (r *queryResolver) RolloutSummary(ctx context.Context, id uuid.UUID) (*model.RolloutSummary, error) {
	return r.Repo.RolloutSummaryGetByID(ctx, id)
}

// Environment is the resolver for the environment field.
func (r *rolloutResolver) Environment(ctx context.Context, obj *model.Rollout) (*model.Environment, error) {
	return r.Repo.EnvironmentCI(ctx, obj.EnvironmentKind)
}

// Events is the resolver for the events field.
func (r *rolloutResolver) Events(ctx context.Context, obj *model.Rollout) ([]*model.RolloutEvent, error) {
	return r.Repo.RolloutEventsGetByRolloutID(ctx, obj.ID)
}

// New is the resolver for the New field.
func (r *rolloutChangesetResolver) New(ctx context.Context, obj *model.RolloutChangeset) (json.RawMessage, error) {
	return json.Marshal(obj.New)
}

// Old is the resolver for the Old field.
func (r *rolloutChangesetResolver) Old(ctx context.Context, obj *model.RolloutChangeset) (json.RawMessage, error) {
	return json.Marshal(obj.Old)
}

// Feature is the resolver for the feature field.
func (r *rolloutSummaryResolver) Feature(ctx context.Context, obj *model.RolloutSummary) (*model.Feature, error) {
	return r.resolveFeatureByName(obj.FeatureName)
}

// Rollouts is the resolver for the rollouts field.
func (r *rolloutSummaryResolver) Rollouts(ctx context.Context, obj *model.RolloutSummary) ([]*model.Rollout, error) {
	return r.Repo.RolloutsBySummaryID(ctx, obj.ID)
}

// Rollout returns graphgen.RolloutResolver implementation.
func (r *Resolver) Rollout() graphgen.RolloutResolver { return &rolloutResolver{r} }

// RolloutChangeset returns graphgen.RolloutChangesetResolver implementation.
func (r *Resolver) RolloutChangeset() graphgen.RolloutChangesetResolver {
	return &rolloutChangesetResolver{r}
}

// RolloutSummary returns graphgen.RolloutSummaryResolver implementation.
func (r *Resolver) RolloutSummary() graphgen.RolloutSummaryResolver {
	return &rolloutSummaryResolver{r}
}

type rolloutResolver struct{ *Resolver }
type rolloutChangesetResolver struct{ *Resolver }
type rolloutSummaryResolver struct{ *Resolver }
