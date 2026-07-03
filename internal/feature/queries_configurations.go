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
)

func environmentConfigurationFromSQL(c featuresql.ConfigurationsEnvironment) *Configuration {
	return &Configuration{
		ID:      c.ID,
		Key:     c.Key,
		Content: c.Value,
		Created: c.Created,
		Source:  ConfigSourceEnv,
	}
}

func globalConfigFromSQL(c featuresql.ConfigurationsGlobal) *Configuration {
	return &Configuration{
		ID:      c.ID,
		Key:     c.Key,
		Content: c.Value,
		Created: c.Created,
		Source:  ConfigSourceGlobal,
	}
}

func GetEnvConfig(ctx context.Context, feature *Feature, envID uuid.UUID) ([]*Configuration, error) {
	globalConfigs, err := querier(ctx).ListGlobalConfigByFeature(ctx, feature.Name)
	if err != nil {
		return nil, err
	}

	envConfigs, err := querier(ctx).ListEnvConfigByFeature(ctx, featuresql.ListEnvConfigByFeatureParams{
		Feature:       feature.Name,
		EnvironmentID: envID,
	})
	if err != nil {
		return nil, err
	}

	merged := MergeConfigs(globalConfigs, envConfigs, nil)
	retVal := make([]*Configuration, 0, len(merged))
	for _, conf := range merged {
		source := ConfigSourceGlobal
		if conf.EnvironmentID != nil {
			source = ConfigSourceEnv
		}
		retVal = append(retVal, &Configuration{
			ID:      conf.ID,
			Key:     conf.Key,
			Content: conf.Value,
			Created: conf.Created,
			Source:  source,
		})
	}

	return retVal, nil
}

func GetGlobalConfig(ctx context.Context, feature string) ([]*Configuration, error) {
	config, err := querier(ctx).ListGlobalConfigByFeature(ctx, feature)
	if err != nil {
		return nil, err
	}
	retVal := make([]*Configuration, 0)
	for _, conf := range config {
		retVal = append(retVal, globalConfigFromSQL(conf))
	}

	return retVal, nil
}

// EnvConfigOverride holds a single environment config override row.
type EnvConfigOverride struct {
	EnvironmentID uuid.UUID
	Key           string
	Content       []byte
}

// ListAllEnvConfigByFeature returns all environment config overrides for a feature.
func ListAllEnvConfigByFeature(ctx context.Context, feature string) ([]EnvConfigOverride, error) {
	rows, err := querier(ctx).ListAllEnvConfigByFeature(ctx, feature)
	if err != nil {
		return nil, err
	}
	result := make([]EnvConfigOverride, len(rows))
	for i, r := range rows {
		result[i] = EnvConfigOverride{
			EnvironmentID: r.EnvironmentID,
			Key:           r.Key,
			Content:       r.Value,
		}
	}
	return result, nil
}

func CreateEnvConfig(ctx context.Context, c NewConfiguration) (*Configuration, error) {
	value, err := json.Marshal(c.Value)
	if err != nil {
		return nil, err
	}

	existing, err := querier(ctx).GetEnvConfigByKey(ctx, featuresql.GetEnvConfigByKeyParams{
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

	// When creating a new env override, use the global value as the "old" value
	// so the activity sidebar shows what the effective value was before.
	var previousVal []byte
	if hadExisting {
		previousVal = existing.Value
	} else {
		global, err := querier(ctx).GetGlobalConfigByKey(ctx, featuresql.GetGlobalConfigByKeyParams{
			Feature: c.Feature,
			Key:     c.Key,
		})
		if err == nil {
			previousVal = global.Value
		}
	}

	if hadExisting && bytes.Equal(existing.Value, value) && stringPtrEqual(existing.Description, c.Description) && existing.Secret == c.Secret {
		return environmentConfigurationFromSQL(existing), nil
	}

	config, err := querier(ctx).UpsertEnvConfig(ctx, featuresql.UpsertEnvConfigParams{
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
		secret:  c.Secret,
		oldVal:  previousVal,
		newVal:  value,
	}); err != nil {
		return nil, err
	}

	return environmentConfigurationFromSQL(config), nil
}

func CreateGlobalConfig(ctx context.Context, c NewConfiguration) (*Configuration, error) {
	value, err := json.Marshal(c.Value)
	if err != nil {
		return nil, err
	}

	existing, err := querier(ctx).GetGlobalConfigByKey(ctx, featuresql.GetGlobalConfigByKeyParams{
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

	config, err := querier(ctx).UpsertGlobalConfig(ctx, featuresql.UpsertGlobalConfigParams{
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
		secret:  c.Secret,
		oldVal:  existingVal(hadExisting, existing.Value),
		newVal:  value,
	}); err != nil {
		return nil, err
	}

	return globalConfigFromSQL(config), nil
}

func UpdateConfig(ctx context.Context, id uuid.UUID, c UpdateConfiguration) (*Configuration, error) {
	var conf featuresql.ConfigurationsGlobal
	err := dbtx.WithTx(ctx, func(ctx context.Context) error {
		var err error
		conf, err = querier(ctx).GetGlobalConfigByID(ctx, id)
		if err != nil {
			return err
		}

		if bytes.Equal(conf.Value, c.Value) && stringPtrEqual(conf.Description, c.Description) {
			return nil
		}

		oldVal := conf.Value

		conf, err = querier(ctx).UpdateGlobalConfig(ctx, featuresql.UpdateGlobalConfigParams{
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
			Feature:    conf.Feature,
			Metadata: configChangeMetadata(configAuditInfo{
				secret: conf.Secret,
				oldVal: oldVal,
				newVal: conf.Value,
			}),
		})
	})
	if err != nil {
		return nil, err
	}

	return globalConfigFromSQL(conf), nil
}

func DeleteConfig(ctx context.Context, id uuid.UUID) error {
	return dbtx.WithTx(ctx, func(ctx context.Context) error {
		existing, err := querier(ctx).GetGlobalConfigByID(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}

		if err := querier(ctx).DeleteGlobalConfig(ctx, id); err != nil {
			return err
		}

		return audit.Create(ctx, audit.CreateParams{
			Action:     audit.ActionDeleted,
			ObjectType: audit.ObjectTypeConfiguration,
			ObjectID:   existing.Feature + "/" + existing.Key,
			Feature:    existing.Feature,
			Metadata: configChangeMetadata(configAuditInfo{
				secret: existing.Secret,
				oldVal: existing.Value,
			}),
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
		Feature:       info.feature,
		EnvironmentID: info.envID,
		Metadata:      configChangeMetadata(info),
	})
}

func UpdateEnvConfig(ctx context.Context, id uuid.UUID, c UpdateConfiguration) (*Configuration, error) {
	var conf featuresql.ConfigurationsEnvironment
	err := dbtx.WithTx(ctx, func(ctx context.Context) error {
		var err error
		conf, err = querier(ctx).GetEnvConfigByID(ctx, id)
		if err != nil {
			return err
		}

		if bytes.Equal(conf.Value, c.Value) && stringPtrEqual(conf.Description, c.Description) {
			return nil
		}

		oldVal := conf.Value

		conf, err = querier(ctx).UpdateEnvConfig(ctx, featuresql.UpdateEnvConfigParams{
			Description: c.Description,
			Value:       c.Value,
			ID:          id,
		})
		if err != nil {
			return err
		}

		return audit.Create(ctx, audit.CreateParams{
			Action:        audit.ActionUpdated,
			ObjectType:    audit.ObjectTypeConfiguration,
			ObjectID:      conf.Feature + "/" + conf.Key,
			Feature:       conf.Feature,
			EnvironmentID: &conf.EnvironmentID,
			Metadata: configChangeMetadata(configAuditInfo{
				secret: conf.Secret,
				oldVal: oldVal,
				newVal: conf.Value,
			}),
		})
	})
	if err != nil {
		return nil, err
	}

	return environmentConfigurationFromSQL(conf), nil
}

func DeleteEnvConfig(ctx context.Context, id uuid.UUID) error {
	return dbtx.WithTx(ctx, func(ctx context.Context) error {
		existing, err := querier(ctx).GetEnvConfigByID(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}

		if err := querier(ctx).DeleteEnvConfig(ctx, id); err != nil {
			return err
		}

		return audit.Create(ctx, audit.CreateParams{
			Action:        audit.ActionDeleted,
			ObjectType:    audit.ObjectTypeConfiguration,
			ObjectID:      existing.Feature + "/" + existing.Key,
			Feature:       existing.Feature,
			EnvironmentID: &existing.EnvironmentID,
			Metadata: configChangeMetadata(configAuditInfo{
				secret: existing.Secret,
				oldVal: existing.Value,
			}),
		})
	})
}

type configAuditInfo struct {
	feature string
	key     string
	envID   *uuid.UUID
	secret  bool
	oldVal  []byte // JSON-encoded, nil if new
	newVal  []byte // JSON-encoded, nil if deleted
}

func configChangeMetadata(info configAuditInfo) map[string]string {
	if info.secret {
		return map[string]string{"secret": "true"}
	}
	m := map[string]string{}
	if info.oldVal != nil {
		m["old"] = string(info.oldVal)
	}
	if info.newVal != nil {
		m["new"] = string(info.newVal)
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

func existingVal(hadExisting bool, val []byte) []byte {
	if !hadExisting {
		return nil
	}
	return val
}

func stringPtrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
