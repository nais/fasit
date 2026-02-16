package database

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/database/gensql"
	"github.com/nais/fasit/internal/graph/model"
)

type EnvironmentValueRepo interface {
	EnvironmentValueDelete(ctx context.Context, environmentID uuid.UUID, key string) error
	EnvironmentValueGet(ctx context.Context, environmentID uuid.UUID, key string, showSensitive bool) (*model.EnvironmentValue, error)
	EnvironmentValuesForEnvironment(ctx context.Context, envID uuid.UUID, showSensitive bool) ([]*model.EnvironmentValue, error)
	EnvironmentValueStore(ctx context.Context, environmentID uuid.UUID, key string, value json.RawMessage, secret bool) error

	EnvironmentValuesAcrossEnvs(ctx context.Context, key string) ([]gensql.EnvironmentValuesAcrossEnvsRow, error)
}

func (r *repo) EnvironmentValueStore(ctx context.Context, environmentID uuid.UUID, key string, value json.RawMessage, secret bool) error {
	err := r.querier.EnvironmentValueStore(ctx, gensql.EnvironmentValueStoreParams{
		Envid:  environmentID,
		Key:    key,
		Value:  value,
		Secret: secret,
	})
	if err != nil {
		return fmt.Errorf("failed to store environment value: %w", err)
	}

	audit.CreateAudit(ctx, "created or updated", "environment_values", environmentID.String()+":"+key)

	return nil
}

func (r *repo) EnvironmentValueGet(ctx context.Context, environmentID uuid.UUID, key string, showSensitive bool) (*model.EnvironmentValue, error) {
	ev, err := r.querier.EnvironmentValueGet(ctx, gensql.EnvironmentValueGetParams{
		Envid:         environmentID,
		Key:           key,
		Showsensitive: showSensitive,
	})
	if err != nil {
		return nil, err
	}

	return &model.EnvironmentValue{
		EnvironmentID: ev.EnvironmentID,
		Key:           ev.Key,
		Value:         ev.Value,
		Secret:        ev.Secret,
	}, nil
}

func (r *repo) EnvironmentValuesForEnvironment(ctx context.Context, envID uuid.UUID, showSensitive bool) ([]*model.EnvironmentValue, error) {
	values, err := r.querier.EnvironmentValuesForEnvironment(ctx, gensql.EnvironmentValuesForEnvironmentParams{
		Envid:         envID,
		Showsensitive: showSensitive,
	})
	if err != nil {
		return nil, err
	}

	ret := make([]*model.EnvironmentValue, len(values))
	for i, ev := range values {
		ret[i] = &model.EnvironmentValue{
			EnvironmentID: ev.EnvironmentID,
			Key:           ev.Key,
			Value:         ev.Value,
			Secret:        ev.Secret,
			KnownUses:     int(ev.Count),
		}
	}

	return ret, nil
}

func (r *repo) EnvironmentValuesAcrossEnvs(ctx context.Context, key string) ([]gensql.EnvironmentValuesAcrossEnvsRow, error) {
	return r.querier.EnvironmentValuesAcrossEnvs(ctx, key)
}

func (r *repo) EnvironmentValueDelete(ctx context.Context, environmentID uuid.UUID, key string) error {
	err := r.querier.EnvironmentValueDelete(ctx, gensql.EnvironmentValueDeleteParams{
		Envid: environmentID,
		Key:   key,
	})
	if err != nil {
		return fmt.Errorf("failed to delete environment value: %w", err)
	}

	audit.CreateAudit(ctx, "deleted", "environment_values", environmentID.String()+":"+key)

	return nil
}
