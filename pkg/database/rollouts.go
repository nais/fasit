package database

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
)

type RolloutRepo interface {
	RolloutByName(ctx context.Context, name string) (*model.Feature, error)
	RolloutByNameAndVersion(ctx context.Context, name, version string) (*model.Rollout, error)
	RolloutCalculateDone(ctx context.Context, rolloutID uuid.UUID) (bool, error)
	RolloutCreate(ctx context.Context, name, version string) (*model.Rollout, error)
	RolloutDelete(ctx context.Context, name string) error
	RolloutEventCreate(ctx context.Context, rollout uuid.UUID, failure bool, message string, data map[string]any) error
	RolloutsForFeature(ctx context.Context, name string) ([]*model.Rollout, error)
	RolloutStatus(ctx context.Context, name string) (model.RolloutStatus, error)
	RolloutsUpdateStatus(ctx context.Context, status model.RolloutStatus, name string, completed bool) error

	RolloutEvents(ctx context.Context, rolloutID uuid.UUID) ([]*model.RolloutEvent, error)
	RolloutMarkFailed(ctx context.Context, rolloutID uuid.UUID) error
}

func (r *repo) RolloutStatus(ctx context.Context, name string) (model.RolloutStatus, error) {
	status, err := r.querier.RolloutStatus(ctx, name)
	if err != nil {
		return "", err
	}

	return model.RolloutStatus(status), nil
}

func (r *repo) RolloutEventCreate(ctx context.Context, rolloutID uuid.UUID, failure bool, message string, data map[string]any) error {
	d := []byte(nil)
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("marshal data: %w", err)
		}

		d = b
	}

	return r.querier.RolloutEventCreate(ctx, gensql.RolloutEventCreateParams{RolloutID: rolloutID, Failure: failure, Message: message, Data: d})
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
		Created:     ro.Created.Time,
		FeatureName: ro.FeatureName,
	}, nil
}

func (r *repo) RolloutByName(ctx context.Context, name string) (*model.Feature, error) {
	f, err := r.querier.RolloutByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get rollout by name from db: %w", err)
	}

	deps := model.Dependencies{}
	if err := json.Unmarshal(f.Dependencies, &deps); err != nil {
		return nil, fmt.Errorf("unmarshal dependencies: %w", err)
	}

	valuesYAML := make(map[string]json.RawMessage)
	if err := json.Unmarshal(f.DefaultValues, &valuesYAML); err != nil {
		return nil, fmt.Errorf("unmarshal default values: %w", err)
	}

	fyaml, defaultValues, err := makeFeatureYAML(f.Kinds, f.Dependencies, f.Values, f.DefaultValues, f.Rename, f.Timeout)
	if err != nil {
		return nil, fmt.Errorf("make feature yaml: %w", err)
	}

	return &model.Feature{
		Name:        f.Name,
		Description: f.Description,
		Version:     f.Version,
		Chart:       f.Chart,
		Source:      f.Source,
		FeatureYAML: fyaml,
		ValuesYAML:  defaultValues,
		SpecVersion: "v2",
		GraphVars: struct {
			EnvironmentID uuid.UUID
			RolloutID     uuid.UUID
		}{
			RolloutID: f.ID,
		},
	}, nil
}

func (r *repo) RolloutsForFeature(ctx context.Context, name string) ([]*model.Rollout, error) {
	rollouts, err := r.querier.RolloutsForFeature(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get rollouts for feature: %w", err)
	}

	var res []*model.Rollout
	for _, ro := range rollouts {
		res = append(res, &model.Rollout{
			ID:          ro.ID,
			Version:     ro.Version,
			Created:     ro.Created.Time,
			FeatureName: ro.FeatureName,
			Completed:   nullTimeToPtr(ro.Completed),
			Status:      model.RolloutStatus(ro.Status),
		})
	}

	return res, nil
}

func (r *repo) RolloutByNameAndVersion(ctx context.Context, name, version string) (*model.Rollout, error) {
	ro, err := r.querier.RolloutByNameAndVersion(ctx, gensql.RolloutByNameAndVersionParams{
		FeatureName: name,
		Version:     version,
	})
	if err != nil {
		return nil, fmt.Errorf("get rollout by name and version: %w", err)
	}

	return &model.Rollout{
		ID:          ro.ID,
		Version:     ro.Version,
		Created:     ro.Created.Time,
		FeatureName: ro.FeatureName,
		Completed:   nullTimeToPtr(ro.Completed),
		Status:      model.RolloutStatus(ro.Status),
	}, nil
}

func (r *repo) RolloutEvents(ctx context.Context, rolloutID uuid.UUID) ([]*model.RolloutEvent, error) {
	events, err := r.querier.RolloutEventForRollout(ctx, rolloutID)
	if err != nil {
		return nil, err
	}

	var res []*model.RolloutEvent
	for _, e := range events {
		var data json.RawMessage
		if e.Data != nil {
			data = json.RawMessage(e.Data)
		}
		res = append(res, &model.RolloutEvent{
			ID:      e.ID,
			Failure: e.Failure,
			Message: e.Message,
			Created: e.Created.Time,
			Data:    data,
		})
	}

	return res, nil
}

func (r *repo) RolloutCalculateDone(ctx context.Context, rolloutID uuid.UUID) (bool, error) {
	return r.querier.RolloutCalculateDone(ctx, rolloutID)
}

func (r *repo) RolloutMarkFailed(ctx context.Context, rolloutID uuid.UUID) error {
	affected, err := r.querier.RolloutMarkFailed(ctx, rolloutID)
	if err != nil {
		return err
	}

	if affected == 0 {
		return fmt.Errorf("rollout already completed")
	}

	return nil
}
