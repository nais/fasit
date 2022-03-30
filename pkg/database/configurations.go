package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
)

var ErrMissingRequiredFields = errors.New("required fields missing")

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
	}, nil
}

func envConfigFromSQL(c gensql.EnvConfigRow) (*model.Configuration, error) {
	return &model.Configuration{
		ID:            c.ID,
		EnvironmentID: nullUUIDToPtr(c.EnvironmentID),
		Feature:       c.Feature,
		Description:   nullStringToPtr(c.Description),
		Key:           c.Key,
		Value:         c.Value,
		Secret:        c.Secret,
		Created:       c.Created,
		Env:           c.Env,
	}, nil
}

func (r *Repo) EnvConfig(ctx context.Context, feature string, envID uuid.UUID) ([]*model.Configuration, error) {
	config, err := r.querier.EnvConfig(ctx, gensql.EnvConfigParams{
		Feature:       feature,
		EnvironmentID: uuid.NullUUID{UUID: envID, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	retVal := []*model.Configuration{}
	for _, conf := range config {
		c, err := envConfigFromSQL(conf)
		if err != nil {
			return nil, err
		}
		retVal = append(retVal, c)
	}

	return retVal, nil
}

func (r *Repo) ConfigGet(ctx context.Context, feature string) ([]*model.Configuration, error) {
	config, err := r.querier.ConfigGet(ctx, feature)
	if err != nil {
		return nil, err
	}
	retVal := []*model.Configuration{}
	for _, conf := range config {
		c, err := configurationFromSQL(conf)
		if err != nil {
			return nil, err
		}
		retVal = append(retVal, c)
	}

	return retVal, nil
}

func (r *Repo) ConfigGetForEnv(ctx context.Context, feature string, envID uuid.UUID) ([]*model.Configuration, error) {
	params := gensql.ConfigGetForEnvParams{
		Feature:       feature,
		EnvironmentID: uuid.NullUUID{UUID: envID, Valid: true},
	}
	config, err := r.querier.ConfigGetForEnv(ctx, params)
	if err != nil {
		return nil, err
	}

	retVal := []*model.Configuration{}
	for _, conf := range config {
		c, err := configurationFromSQL(conf)
		if err != nil {
			return nil, err
		}
		retVal = append(retVal, c)
	}

	return retVal, nil
}

func (r *Repo) ConfigCreate(ctx context.Context, c model.NewConfiguration) (*model.Configuration, error) {
	value, err := json.Marshal(c.Value)
	if err != nil {
		return nil, err
	}

	envID := uuid.Nil
	if c.EnvironmentID != nil {
		envID = *c.EnvironmentID
	}

	config, err := r.querier.ConfigUpdateOrCreate(ctx, gensql.ConfigUpdateOrCreateParams{
		EnvironmentID: uuid.NullUUID{UUID: envID, Valid: true},
		Feature:       c.Feature,
		Description:   ptrToNullString(c.Description),
		Secret:        c.Secret,
		Key:           c.Key,
		Value:         value,
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

func (r *Repo) ConfigUpdate(ctx context.Context, id uuid.UUID, c model.UpdateConfiguration) (*model.Configuration, error) {
	conf, err := r.querier.ConfigUpdate(ctx, gensql.ConfigUpdateParams{
		Description: ptrToNullString(c.Description),
		Value:       c.Value,
		ID:          id,
	})
	if err != nil {
		return nil, err
	}
	return configurationFromSQL(conf)
}

func (r *Repo) ConfigDelete(ctx context.Context, id uuid.UUID) error {
	return r.querier.ConfigDelete(ctx, id)
}

func (r *Repo) HelmValues(ctx context.Context, feature string, envID uuid.UUID, requiredFields []string) (map[string]any, error) {
	vals, err := r.querier.ConfigForEnv(ctx, gensql.ConfigForEnvParams{
		Feature:       feature,
		EnvironmentID: envID,
	})
	if err != nil {
		return nil, err
	}

	i := 0
	for _, key := range vals {
		for _, req := range requiredFields {
			if key.Key == req {
				i++
			}
		}
	}
	if i != len(requiredFields) {
		return nil, ErrMissingRequiredFields
	}

	return makeHelmConfigMap(vals)
}

func makeHelmConfigMap(vals []gensql.ConfigForEnvRow) (map[string]any, error) {
	val := make(map[string]any)

	for _, v := range vals {
		keys := strings.Split(v.Key, ".")
		parent := val
		for index, key := range keys {
			if index == len(keys)-1 {
				parent[key] = v.Value
				continue
			}
			if e, ok := parent[key]; ok {
				if p, ok := e.(map[string]any); ok {
					parent = p
					continue
				}
				return nil, fmt.Errorf("key %v is not nestable", v.Key)
			}
			f := make(map[string]any)
			parent[key] = f
			parent = f
		}
	}
	return val, nil
}
