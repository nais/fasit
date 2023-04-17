package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
)

type RolloutRepo interface {
	RolloutByName(ctx context.Context, name string) (*model.Feature, error)
	RolloutCreate(ctx context.Context, name, version string) (*model.Rollout, error)
	RolloutDelete(ctx context.Context, name string) error
	RolloutEventCreate(ctx context.Context, rollout uuid.UUID, failure bool, message string) error
	RolloutsListen(ctx context.Context, fn ListenFunc) error
	RolloutStatus(ctx context.Context, name string) (model.RolloutStatus, error)
	RolloutsUpdateStatus(ctx context.Context, status model.RolloutStatus, name string, completed bool) error
}

func (r *repo) RolloutComplete(ctx context.Context, name string) error {
	return r.querier.RolloutComplete(ctx, name)
}

func (r *repo) RolloutStatus(ctx context.Context, name string) (model.RolloutStatus, error) {
	status, err := r.querier.RolloutStatus(ctx, name)
	if err != nil {
		return "", err
	}

	return model.RolloutStatus(status), nil
}

func (r *repo) RolloutEventCreate(ctx context.Context, rolloutID uuid.UUID, failure bool, message string) error {
	return r.querier.RolloutEventCreate(ctx, gensql.RolloutEventCreateParams{RolloutID: rolloutID, Failure: failure, Message: message})
}

func (r *repo) RolloutsUpdateStatus(ctx context.Context, status model.RolloutStatus, name string, completed bool) error {
	if err := r.querier.RolloutUpdateStatus(ctx, gensql.RolloutUpdateStatusParams{Status: status.String(), FeatureName: name}); err != nil {
		return fmt.Errorf("update rollout status: %w", err)
	}
	if completed {
		if err := r.querier.RolloutComplete(ctx, name); err != nil {
			return fmt.Errorf("complete rollout: %w", err)
		}
	}

	return nil
}

func (r *repo) RolloutDelete(ctx context.Context, name string) error {
	return r.querier.RolloutDelete(ctx, name)
}

func (r *repo) RolloutCreate(ctx context.Context, name, version string) (*model.Rollout, error) {
	ro, err := r.querier.RolloutCreate(ctx, gensql.RolloutCreateParams{FeatureName: name, Version: version})
	if err != nil {
		return nil, err
	}

	return &model.Rollout{
		ID:          ro.ID,
		Version:     ro.Version,
		Created:     ro.Created,
		FeatureName: ro.FeatureName,
	}, nil
}

func (r *repo) RolloutsListen(ctx context.Context, fn ListenFunc) error {
	return r.ListenNotify(ctx, "rollout_notify", fn)
}

func (r *repo) RolloutByName(ctx context.Context, name string) (*model.Feature, error) {
	f, err := r.querier.RolloutByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get feature by name from db: %w", err)
	}

	deps := model.Dependencies{}
	if err := json.Unmarshal(f.Dependencies.Bytes, &deps); err != nil {
		return nil, fmt.Errorf("unmarshal dependencies: %w", err)
	}

	valuesYAML := make(map[string]any)
	if err := json.Unmarshal(f.DefaultValues.Bytes, &valuesYAML); err != nil {
		return nil, fmt.Errorf("unmarshal default values: %w", err)
	}

	return &model.Feature{
		Name:        f.Name,
		Description: f.Description,
		Version:     f.Version,
		Chart:       f.Chart,
		Source:      f.Source,
		FeatureYAML: model.FeatureYAML{
			Dependencies: deps,
			Timeout:      time.Duration(f.Timeout.Int64) * time.Microsecond,
		},
		ValuesYAML: valuesYAML,
		GraphVars: struct {
			EnvironmentID uuid.UUID
			RolloutID     uuid.UUID
		}{
			RolloutID: f.ID,
		},
	}, nil
}
