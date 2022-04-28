package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
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
		Env:           c.EnvironmentID.Valid && c.EnvironmentID.UUID != uuid.Nil,
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

func (r *repo) EnvConfig(ctx context.Context, feature string, envID uuid.UUID) ([]*model.Configuration, error) {
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

func (r *repo) ConfigGet(ctx context.Context, feature string) ([]*model.Configuration, error) {
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

func (r *repo) ConfigGetForEnv(ctx context.Context, feature string, envID uuid.UUID) ([]*model.Configuration, error) {
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

func (r *repo) ConfigCreate(ctx context.Context, c model.NewConfiguration) (*model.Configuration, error) {
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

func (r *repo) ConfigUpdate(ctx context.Context, id uuid.UUID, c model.UpdateConfiguration) (*model.Configuration, error) {
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

func (r *repo) ConfigDelete(ctx context.Context, id uuid.UUID) error {
	return r.querier.ConfigDelete(ctx, id)
}

func (r *repo) HelmValues(ctx context.Context, feature string, envID uuid.UUID, requiredFields []string) (map[string]any, error) {
	vals, err := r.querier.ConfigForEnv(ctx, gensql.ConfigForEnvParams{
		Feature:       feature,
		EnvironmentID: envID,
	})
	if err != nil {
		return nil, err
	}

	missing := validateFields(requiredFields, vals)
	if len(missing) > 0 {
		return nil, &ErrMissingRequiredFields{Fields: missing}
	}
	return makeHelmConfigMap(vals)
}

func validateFields(requiredFields []string, values []gensql.ConfigForEnvRow) []string {
	fields := map[string]int{}
	for _, req := range requiredFields {
		fields[req] = 0
		for _, k := range values {
			if k.Key == req {
				fields[req] = 1
			}
		}
	}

	var missing []string
	for k, v := range fields {
		if v == 0 {
			missing = append(missing, k)
		}
	}
	return missing
}

func makeHelmConfigMap(vals []gensql.ConfigForEnvRow) (map[string]any, error) {
	val := make(map[string]any)

	for _, v := range vals {
		keys, err := smartDotSplit(v.Key)
		if err != nil {
			return nil, err
		}
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

func smartDotSplit(s string) ([]string, error) {
	if strings.HasSuffix(s, ".") {
		return nil, fmt.Errorf("cannot end with `.`")
	}
	if strings.HasPrefix(s, ".") {
		return nil, fmt.Errorf("cannot start with `.`")
	}

	str := ""
	var ret []string
	for i, ch := range s {
		switch ch {
		case '.':
			if len(str) == 0 || i == 0 {
				return nil, fmt.Errorf("invalid `.` on position %v", i)
			}
			if s[i-1] == '\\' {
				str = str[:len(str)-1]
				str += "."
			} else {
				ret = append(ret, str)
				str = ""
			}
		default:
			str += string(ch)
		}
	}
	return append(ret, str), nil
}
