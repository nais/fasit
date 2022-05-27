package database

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
)

func (r *repo) EnvironmentValueStore(ctx context.Context, environmentID uuid.UUID, key string, value json.RawMessage) error {
	return r.querier.EnvironmentValueStore(ctx, gensql.EnvironmentValueStoreParams{
		Envid: environmentID,
		Key:   key,
		Value: value,
	})
}

func (r *repo) EnvironmentValueGet(ctx context.Context, environmentID uuid.UUID, key string) (*model.EnvironmentValue, error) {
	ev, err := r.querier.EnvironmentValueGet(ctx, gensql.EnvironmentValueGetParams{
		Envid: environmentID,
		Key:   key,
	})
	if err != nil {
		return nil, err
	}

	return environmentValueFromSQL(ev), nil
}

func environmentValueFromSQL(p gensql.EnvironmentValue) *model.EnvironmentValue {
	return &model.EnvironmentValue{
		EnvironmentID: p.EnvironmentID,
		Key:           p.Key,
		Value:         p.Value,
	}
}
