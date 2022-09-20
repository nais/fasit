package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph/model"
)

type ConfigRepo interface {
	ConfigCreate(ctx context.Context, c model.NewConfiguration) (model.Configuration, error)
	ConfigDelete(ctx context.Context, id uuid.UUID) error
	ConfigGet(ctx context.Context, feature string) ([]*model.GlobalConfiguration, error)
	ConfigGetForEnv(ctx context.Context, feature string, envID uuid.UUID) ([]*model.EnvConfiguration, error)
	ConfigListen(ctx context.Context, fn ListenFunc) error
	ConfigUpdate(ctx context.Context, id uuid.UUID, c model.UpdateConfiguration) (model.Configuration, error)
	EnvConfig(ctx context.Context, feature string, envID uuid.UUID) ([]model.Configuration, error)
	HelmValues(ctx context.Context, feature feature.Feature, envID uuid.UUID, requiredFields []string) (values map[string]any, rolloutIDs []uuid.UUID, err error)
}

func environmentConfigurationFromSQL(c gensql.ConfigurationsEnvironment) *model.EnvConfiguration {
	return &model.EnvConfiguration{
		ID:            c.ID,
		EnvironmentID: c.EnvironmentID,
		FeatureName:   c.Feature,
		// Description:   nullStringToPtr(c.Description),
		Key:     c.Key,
		Value:   c.Value,
		Secret:  c.Secret,
		Created: c.Created,
	}
}

func globalConfigFromSQL(c gensql.ConfigurationsGlobal) *model.GlobalConfiguration {
	return &model.GlobalConfiguration{
		ID:          c.ID,
		FeatureName: c.Feature,
		// Description: nullStringToPtr(c.Description),
		Key:     c.Key,
		Value:   c.Value,
		Secret:  c.Secret,
		Created: c.Created,
	}
}

func (r *repo) EnvConfig(ctx context.Context, feature string, envID uuid.UUID) ([]model.Configuration, error) {
	config, err := r.querier.EnvConfig(ctx, gensql.EnvConfigParams{
		Feature:       feature,
		EnvironmentID: envID,
	})
	if err != nil {
		return nil, err
	}
	retVal := []model.Configuration{}
	for _, conf := range config {
		retVal = append(retVal, envConfigFromSQL(conf))
	}

	return retVal, nil
}

func envConfigFromSQL(conf gensql.EnvConfigRow) model.Configuration {
	if conf.EnvironmentID.Valid {
		return &model.EnvConfiguration{
			ID:            conf.ID,
			EnvironmentID: conf.EnvironmentID.UUID,
			FeatureName:   conf.Feature,
			// Description:   nullStringToPtr(conf.Description),
			Key:   conf.Key,
			Value: conf.Value,
		}
	}

	return &model.GlobalConfiguration{
		ID:          conf.ID,
		FeatureName: conf.Feature,
		// Description: nullStringToPtr(conf.Description),
		Key:   conf.Key,
		Value: conf.Value,
	}
}

func (r *repo) ConfigGet(ctx context.Context, feature string) ([]*model.GlobalConfiguration, error) {
	config, err := r.querier.ConfigGet(ctx, feature)
	if err != nil {
		return nil, err
	}
	retVal := []*model.GlobalConfiguration{}
	for _, conf := range config {
		retVal = append(retVal, globalConfigFromSQL(conf))
	}

	return retVal, nil
}

func (r *repo) ConfigGetForEnv(ctx context.Context, feature string, envID uuid.UUID) ([]*model.EnvConfiguration, error) {
	params := gensql.ConfigGetForEnvParams{
		Feature:       feature,
		EnvironmentID: envID,
	}
	config, err := r.querier.ConfigGetForEnv(ctx, params)
	if err != nil {
		return nil, err
	}

	retVal := []*model.EnvConfiguration{}
	for _, conf := range config {
		retVal = append(retVal, environmentConfigurationFromSQL(conf))
	}

	return retVal, nil
}

func (r *repo) ConfigCreate(ctx context.Context, c model.NewConfiguration) (model.Configuration, error) {
	value, err := json.Marshal(c.Value)
	if err != nil {
		return nil, err
	}

	if c.EnvironmentID != nil && *c.EnvironmentID != uuid.Nil {
		return r.configEnvCreate(ctx, c, value)
	}

	return r.configGlobalCreate(ctx, c, value)
}

func (r *repo) configEnvCreate(ctx context.Context, c model.NewConfiguration, value []byte) (*model.EnvConfiguration, error) {
	config, err := r.querier.ConfigEnvUpdateOrCreate(ctx, gensql.ConfigEnvUpdateOrCreateParams{
		EnvironmentID: *c.EnvironmentID,
		Feature:       c.Feature,
		Description:   ptrToNullString(c.Description),
		Secret:        c.Secret,
		Key:           c.Key,
		Value:         value,
		RolloutID:     ptrToNullUUID(c.RolloutID),
	})
	if err != nil {
		return nil, err
	}

	return environmentConfigurationFromSQL(config), nil
}

func (r *repo) configGlobalCreate(ctx context.Context, c model.NewConfiguration, value []byte) (*model.GlobalConfiguration, error) {
	config, err := r.querier.ConfigGlobalUpdateOrCreate(ctx, gensql.ConfigGlobalUpdateOrCreateParams{
		Feature:     c.Feature,
		Description: ptrToNullString(c.Description),
		Secret:      c.Secret,
		Key:         c.Key,
		Value:       value,
		RolloutID:   ptrToNullUUID(c.RolloutID),
	})
	if err != nil {
		return nil, err
	}

	return globalConfigFromSQL(config), nil
}

func (r *repo) ConfigUpdate(ctx context.Context, id uuid.UUID, c model.UpdateConfiguration) (model.Configuration, error) {
	conf, err := r.querier.ConfigUpdate(ctx, gensql.ConfigUpdateParams{
		Description: ptrToNullString(c.Description),
		Value:       c.Value,
		ID:          id,
	})
	if err != nil {
		return nil, err
	}
	return globalConfigFromSQL(conf), nil
}

func (r *repo) ConfigDelete(ctx context.Context, id uuid.UUID) error {
	return r.querier.ConfigDelete(ctx, id)
}

func (r *repo) HelmValues(ctx context.Context, feature feature.Feature, envID uuid.UUID, requiredFields []string) (map[string]any, []uuid.UUID, error) {
	mv, err := r.MappingValuesForEnvironment(ctx, envID, true)
	if err != nil {
		return nil, nil, err
	}

	vals, err := r.querier.EnvConfig(ctx, gensql.EnvConfigParams{
		Feature:       feature.Name,
		EnvironmentID: envID,
	})
	if err != nil {
		return nil, nil, err
	}

	missing := validateFields(requiredFields, vals)
	if len(missing) > 0 {
		return nil, nil, &ErrMissingRequiredFields{Fields: missing}
	}
	mp, err := makeHelmConfigMap(vals)
	if err != nil {
		return nil, nil, err
	}

	var rolloutIDs []uuid.UUID
	ridUnique := map[uuid.UUID]struct{}{}
	for _, v := range vals {
		if v.RolloutID.Valid {
			if _, ok := ridUnique[v.RolloutID.UUID]; !ok {
				ridUnique[v.RolloutID.UUID] = struct{}{}
				rolloutIDs = append(rolloutIDs, v.RolloutID.UUID)
			}
		}
	}

	err = feature.Mapping.Generate(mv, mp)
	return mp, rolloutIDs, err
}

func validateFields(requiredFields []string, values []gensql.EnvConfigRow) []string {
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

func makeHelmConfigMap(vals []gensql.EnvConfigRow) (map[string]any, error) {
	val := make(map[string]any)

	for _, v := range vals {
		keys, err := feature.SmartDotSplit(v.Key)
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
					if index == len(keys)-1 {
						return nil, fmt.Errorf("key %v is not nestable", v.Key)
					}
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

func (r *repo) ConfigListen(ctx context.Context, fn ListenFunc) error {
	listener := pq.NewListener(r.dbConnDSN, time.Millisecond, 15*time.Second, nil)

	if err := listener.Listen("configurations_notify"); err != nil {
		return err
	}

	defer listener.Close()

	for {
		select {
		case <-ctx.Done():
			return listener.UnlistenAll()
		case n := <-listener.Notify:
			if n == nil {
				continue
			}

			id, err := uuid.Parse(n.Extra)
			if err != nil {
				r.log.WithField("query", "configurations_listen").WithError(err).Warn("invalid uuid")
				continue
			}

			fn(ctx, id)
		}
	}
}
