package feature

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/feature/featuresql"
	"github.com/nais/fasit/internal/graph/model"
)

func environmentConfigurationFromSQL(c featuresql.ConfigurationsEnvironment) *model.Configuration {
	return &model.Configuration{
		ID: c.ID,
		GraphVars: model.ConfigurationGraphVars{
			EnvironmentID: &c.EnvironmentID,
			FeatureName:   c.Feature,
		},
		Key:     c.Key,
		Content: c.Value,
		Created: c.Created.Time,
		Source:  model.ConfigSourceEnv,
	}
}

func globalConfigFromSQL(c featuresql.ConfigurationsGlobal) *model.Configuration {
	return &model.Configuration{
		ID:        c.ID,
		GraphVars: model.ConfigurationGraphVars{FeatureName: c.Feature},
		Key:       c.Key,
		Content:   c.Value,
		Created:   c.Created.Time,
		Source:    model.ConfigSourceGlobal,
	}
}

func EnvConfig(ctx context.Context, feature *model.Feature, envID uuid.UUID) ([]*model.Configuration, error) {
	config, err := querier(ctx).EnvConfig(ctx, featuresql.EnvConfigParams{
		Feature:       feature.Name,
		EnvironmentID: envID,
	})
	if err != nil {
		return nil, err
	}
	retVal := []*model.Configuration{}
	knownKeys := make(map[string]struct{}, len(config))
	for _, conf := range config {
		retVal = append(retVal, envConfigFromSQL(conf))
		knownKeys[conf.Key] = struct{}{}
	}

	for _, m := range feature.Rename {
		for _, rv := range retVal {
			if rv.Key == m.From {
				_, ok := knownKeys[m.To]
				if !ok {
					rv.Key = m.To
				}
				break
			}
		}
	}

	return retVal, nil
}

func envConfigFromSQL(conf featuresql.EnvConfigRow) *model.Configuration {
	return &model.Configuration{
		ID: conf.ID,
		GraphVars: model.ConfigurationGraphVars{
			EnvironmentID: conf.EnvironmentID,
			FeatureName:   conf.Feature,
		},
		Key:     conf.Key,
		Content: conf.Value,
		Source:  model.ConfigSourceEnv,
	}
}

func ConfigGet(ctx context.Context, feature string) ([]*model.Configuration, error) {
	config, err := querier(ctx).ConfigGet(ctx, feature)
	if err != nil {
		return nil, err
	}
	retVal := []*model.Configuration{}
	for _, conf := range config {
		retVal = append(retVal, globalConfigFromSQL(conf))
	}

	return retVal, nil
}

func ConfigCreate(ctx context.Context, c model.NewConfiguration) (*model.Configuration, error) {
	value, err := json.Marshal(c.Value)
	if err != nil {
		return nil, err
	}

	if c.EnvironmentID != nil && *c.EnvironmentID != uuid.Nil {
		return configEnvCreate(ctx, c, value)
	}

	return configGlobalCreate(ctx, c, value)
}

func configEnvCreate(ctx context.Context, c model.NewConfiguration, value []byte) (*model.Configuration, error) {
	config, err := querier(ctx).ConfigEnvUpdateOrCreate(ctx, featuresql.ConfigEnvUpdateOrCreateParams{
		EnvironmentID: *c.EnvironmentID,
		Feature:       c.Feature,
		Description:   c.Description,
		Secret:        c.Secret,
		Key:           c.Key,
		Value:         value,
	})
	if err != nil {
		return nil, err
	}

	audit.CreateAudit(ctx, "config created or updated", "configurations_environment", config.ID.String())

	return environmentConfigurationFromSQL(config), nil
}

func configGlobalCreate(ctx context.Context, c model.NewConfiguration, value []byte) (*model.Configuration, error) {
	config, err := querier(ctx).ConfigGlobalUpdateOrCreate(ctx, featuresql.ConfigGlobalUpdateOrCreateParams{
		Feature:     c.Feature,
		Description: c.Description,
		Secret:      c.Secret,
		Key:         c.Key,
		Value:       value,
	})
	if err != nil {
		return nil, err
	}

	audit.CreateAudit(ctx, "config created or updated", "configurations_global", config.ID.String())

	return globalConfigFromSQL(config), nil
}

func ConfigUpdate(ctx context.Context, id uuid.UUID, c model.UpdateConfiguration) (*model.Configuration, error) {
	conf, err := querier(ctx).ConfigUpdate(ctx, featuresql.ConfigUpdateParams{
		Description: c.Description,
		Value:       c.Value,
		ID:          id,
	})
	if err != nil {
		return nil, err
	}

	audit.CreateAudit(ctx, "config updated", "configurations_global", conf.ID.String())

	return globalConfigFromSQL(conf), nil
}

func ConfigDelete(ctx context.Context, id uuid.UUID) error {
	if err := querier(ctx).ConfigDelete(ctx, id); err != nil {
		return err
	}

	audit.CreateAudit(ctx, "config deleted", "configurations_global", id.String())
	return nil
}

func ConfigMove(ctx context.Context, feature string, moves []model.Rename) error {
	for _, m := range moves {
		err := querier(ctx).ConfigRenameEnv(ctx, featuresql.ConfigRenameEnvParams{
			FromKey: m.From,
			ToKey:   m.To,
			Feature: feature,
		})
		if err != nil {
			return fmt.Errorf("rename env config: %w", err)
		}

		err = querier(ctx).ConfigRenameGlobal(ctx, featuresql.ConfigRenameGlobalParams{
			FromKey: m.From,
			ToKey:   m.To,
			Feature: feature,
		})
		if err != nil {
			return fmt.Errorf("rename global config: %w", err)
		}
	}

	return nil
}

func ConfigOverridesByFeature(ctx context.Context, featureName string) ([]*model.ConfigOverride, error) {
	overrides, err := querier(ctx).ConfigOverridesByFeature(ctx, featureName)
	if err != nil {
		return nil, err
	}

	result := make([]*model.ConfigOverride, len(overrides))
	for i, o := range overrides {
		result[i] = &model.ConfigOverride{
			EnvironmentID: o.EnvironmentID,
			Keys:          o.Keys,
		}
	}
	return result, nil
}

func ConfigGetByID(ctx context.Context, id uuid.UUID) (*model.Configuration, error) {
	config, err := querier(ctx).ConfigGetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return globalConfigFromSQL(config), nil
}
