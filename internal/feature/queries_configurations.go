package feature

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/dbtx"
	"github.com/nais/fasit/internal/feature/featuresql"
	"github.com/nais/fasit/internal/graph/model"
)

func environmentConfigurationFromSQL(c featuresql.ConfigurationsEnvironment) *model.Configuration {
	return &model.Configuration{
		ID:      c.ID,
		Key:     c.Key,
		Content: c.Value,
		Created: c.Created.Time,
		Source:  model.ConfigSourceEnv,
	}
}

func globalConfigFromSQL(c featuresql.ConfigurationsGlobal) *model.Configuration {
	return &model.Configuration{
		ID:      c.ID,
		Key:     c.Key,
		Content: c.Value,
		Created: c.Created.Time,
		Source:  model.ConfigSourceGlobal,
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

	return retVal, nil
}

func envConfigFromSQL(conf featuresql.EnvConfigRow) *model.Configuration {
	return &model.Configuration{
		ID:      conf.ID,
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
	existing, err := querier(ctx).ConfigEnvGet(ctx, featuresql.ConfigEnvGetParams{
		EnvironmentID: *c.EnvironmentID,
		Feature:       c.Feature,
		Key:           c.Key,
	})
	hadExisting := true
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		hadExisting = false
	}

	if hadExisting && bytes.Equal(existing.Value, value) && stringPtrEqual(existing.Description, c.Description) && existing.Secret == c.Secret {
		return environmentConfigurationFromSQL(existing), nil
	}

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

	if err := writeConfigUpsertAudit(ctx, hadExisting, configAuditInfo{
		feature: config.Feature,
		key:     config.Key,
		envID:   &config.EnvironmentID,
	}); err != nil {
		return nil, err
	}

	return environmentConfigurationFromSQL(config), nil
}

func configGlobalCreate(ctx context.Context, c model.NewConfiguration, value []byte) (*model.Configuration, error) {
	existing, err := querier(ctx).ConfigGlobalGetByKey(ctx, featuresql.ConfigGlobalGetByKeyParams{
		Feature: c.Feature,
		Key:     c.Key,
	})
	hadExisting := true
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		hadExisting = false
	}

	if hadExisting && bytes.Equal(existing.Value, value) && stringPtrEqual(existing.Description, c.Description) && existing.Secret == c.Secret {
		return globalConfigFromSQL(existing), nil
	}

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

	if err := writeConfigUpsertAudit(ctx, hadExisting, configAuditInfo{
		feature: config.Feature,
		key:     config.Key,
		envID:   nil,
	}); err != nil {
		return nil, err
	}

	return globalConfigFromSQL(config), nil
}

func ConfigUpdate(ctx context.Context, id uuid.UUID, c model.UpdateConfiguration) (*model.Configuration, error) {
	var conf featuresql.ConfigurationsGlobal
	err := dbtx.WithTx(ctx, func(ctx context.Context) error {
		var err error
		conf, err = querier(ctx).ConfigGetByID(ctx, id)
		if err != nil {
			return err
		}

		if bytes.Equal(conf.Value, c.Value) && stringPtrEqual(conf.Description, c.Description) {
			return nil
		}

		conf, err = querier(ctx).ConfigUpdate(ctx, featuresql.ConfigUpdateParams{
			Description: c.Description,
			Value:       c.Value,
			ID:          id,
		})
		if err != nil {
			return err
		}

		return audit.Create(ctx, audit.CreateParams{
			Action:     audit.ActionUpdated,
			ObjectType: audit.ObjectTypeConfiguration,
			ObjectID:   conf.Feature + "/" + conf.Key,
		})
	})
	if err != nil {
		return nil, err
	}

	return globalConfigFromSQL(conf), nil
}

func ConfigDelete(ctx context.Context, id uuid.UUID) error {
	return dbtx.WithTx(ctx, func(ctx context.Context) error {
		existing, err := querier(ctx).ConfigGetByID(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}

		if err := querier(ctx).ConfigDelete(ctx, id); err != nil {
			return err
		}

		return audit.Create(ctx, audit.CreateParams{
			Action:     audit.ActionDeleted,
			ObjectType: audit.ObjectTypeConfiguration,
			ObjectID:   existing.Feature + "/" + existing.Key,
		})
	})
}

func writeConfigUpsertAudit(ctx context.Context, hadExisting bool, info configAuditInfo) error {
	action := audit.ActionCreated
	if hadExisting {
		action = audit.ActionUpdated
	}

	return audit.Create(ctx, audit.CreateParams{
		Action:        action,
		ObjectType:    audit.ObjectTypeConfiguration,
		ObjectID:      info.feature + "/" + info.key,
		EnvironmentID: info.envID,
	})
}

type configAuditInfo struct {
	feature string
	key     string
	envID   *uuid.UUID
}

func stringPtrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
