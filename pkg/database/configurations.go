package database

import (
	"context"
	"encoding/json"

	"github.com/nais/c3po/pkg/database/gensql"
	"github.com/nais/c3po/pkg/graph/model"
)

func configurationFromSQL(c gensql.Configuration) (*model.Configuration, error) {
	return &model.Configuration{
		ID:            c.ID,
		EnvironmentID: nullUUIDToPtr(c.EnvironmentID),
		Feature:       c.Feature,
		Description:   nullStringToPtr(c.Description),
		Key:           c.Key,
		Value:         c.Value,
		Secret:        c.Secret,
		Created:       c.Created,
		Deleted:       c.Deleted.Bool,
	}, nil
}

func (r *Repo) ConfigGet(ctx context.Context, feature string) (*model.Configuration, error) {
	config, err := r.querier.ConfigGet(ctx, feature)
	if err != nil {
		return nil, err
	}

	c, err := configurationFromSQL(config)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (r *Repo) ConfigCreate(ctx context.Context, c model.NewConfiguration) (*model.Configuration, error) {
	value, err := json.Marshal(c.Value)
	if err != nil {
		return nil, err
	}

	config, err := r.querier.ConfigCreate(ctx, gensql.ConfigCreateParams{
		EnvironmentID: ptrToNullUUID(c.EnvironmentID),
		Feature:       c.Feature,
		Description:   ptrToNullString(c.Description),
		Secret:        c.Secret,
		Key:           c.Key,
		Value:         json.RawMessage(value),
	})
	if err != nil {
		return nil, err
	}

	ret, err := configurationFromSQL(config)
	if err != nil {
		return nil, err
	}

	return ret, nil
}
