package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v4"
	"github.com/nais/fasit/pkg/graph/graphgen"
	"github.com/nais/fasit/pkg/graph/model"
)

// FeatureStatus is the resolver for the featureStatus field.
func (r *queryResolver) FeatureStatus(ctx context.Context, envID uuid.UUID, feature string) (*model.Status, error) {
	status, err := r.Repo.StatusForFeature(ctx, envID, feature)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			f := r.Resolver.Features.Get(feature)
			if f == nil {
				return nil, fmt.Errorf("feature %v not found", feature)
			}
			return &model.Status{
				EnvironmentID: envID,
				Feature:       feature,
				Version:       f.Version,
				Status:        model.RolloutStatusUnknown,
				Created:       time.Now(),
				LastModified:  time.Now(),
				Log:           "",
			}, nil
		}
		return nil, err
	}
	return status, nil
}

// MissingDependencies is the resolver for the missingDependencies field.
func (r *statusResolver) MissingDependencies(ctx context.Context, obj *model.Status) ([]*model.Feature, error) {
	f := r.Features.Get(obj.Feature)

	states, err := r.Repo.FeatureStatesGet(ctx, obj.EnvironmentID)
	if err != nil {
		return nil, err
	}

	enabledFeatures := []string{}
	for _, s := range states {
		if s.Enabled && s.RolloutStatus == model.RolloutStatusDeployed {
			enabledFeatures = append(enabledFeatures, s.FeatureName)
		}
	}

	ret := []*model.Feature{}

	for _, d := range f.DependsOn.FindMissing(enabledFeatures) {
		feat := r.Features.Get(d)
		if feat == nil {
			return nil, fmt.Errorf("invalid dependency %v", d)
		}
		f, err := marshalFeature(*feat)
		if err != nil {
			return nil, err
		}
		ret = append(ret, f)
	}
	return ret, nil
}

// Status is the resolver for the status field.
func (r *subscriptionResolver) Status(ctx context.Context, envID uuid.UUID, feature string) (<-chan *model.Status, error) {
	ch := make(chan *model.Status, 1)
	go func() {
		defer close(ch)
		for {
			s, err := r.Repo.StatusForFeature(ctx, envID, feature)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					r.Log.WithError(err).Debug("no rows returned from database")
					return
				}
				r.Log.WithError(err).Warn("failed to read status from status subscription")
				return
			}
			ch <- s
			select {
			case <-ctx.Done():
				r.Log.Debug("closing status subscription")
				return
			case <-time.After(5 * time.Second):
				// continue loop
			}
		}
	}()
	return ch, nil
}

// Status returns graphgen.StatusResolver implementation.
func (r *Resolver) Status() graphgen.StatusResolver { return &statusResolver{r} }

// Subscription returns graphgen.SubscriptionResolver implementation.
func (r *Resolver) Subscription() graphgen.SubscriptionResolver { return &subscriptionResolver{r} }

type statusResolver struct{ *Resolver }
type subscriptionResolver struct{ *Resolver }
