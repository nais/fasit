package feature

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
		objectType: "configurations_environment",
		objectID:   config.ID.String(),
		feature:    config.Feature,
		key:        config.Key,
		envID:      &config.EnvironmentID,
		secret:     config.Secret,
		before:     existing.Value,
		after:      value,
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
		objectType: "configurations_global",
		objectID:   config.ID.String(),
		feature:    config.Feature,
		key:        config.Key,
		envID:      nil,
		secret:     config.Secret,
		before:     existing.Value,
		after:      value,
	}); err != nil {
		return nil, err
	}

	return globalConfigFromSQL(config), nil
}

func ConfigUpdate(ctx context.Context, id uuid.UUID, c model.UpdateConfiguration) (*model.Configuration, error) {
	existing, err := querier(ctx).ConfigGetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if bytes.Equal(existing.Value, c.Value) && stringPtrEqual(existing.Description, c.Description) {
		return globalConfigFromSQL(existing), nil
	}

	conf, err := querier(ctx).ConfigUpdate(ctx, featuresql.ConfigUpdateParams{
		Description: c.Description,
		Value:       c.Value,
		ID:          id,
	})
	if err != nil {
		return nil, err
	}

	if err := audit.Create(ctx, audit.CreateParams{
		Description: configDescription("updated", conf.Key, conf.Secret),
		ObjectType:  "configurations_global",
		ObjectID:    conf.ID.String(),
		Metadata: configMetadata("update", configMetadataInput{
			feature: conf.Feature,
			key:     conf.Key,
			envID:   nil,
			secret:  conf.Secret,
			before:  existing.Value,
			after:   c.Value,
		}),
	}); err != nil {
		return nil, err
	}

	return globalConfigFromSQL(conf), nil
}

func ConfigDelete(ctx context.Context, id uuid.UUID) error {
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
		Description: configDescription("deleted", existing.Key, existing.Secret),
		ObjectType:  "configurations_global",
		ObjectID:    id.String(),
		Metadata: configMetadata("delete", configMetadataInput{
			feature: existing.Feature,
			key:     existing.Key,
			envID:   nil,
			secret:  existing.Secret,
			before:  existing.Value,
		}),
	})
}

type configAuditInfo struct {
	objectType string
	objectID   string
	feature    string
	key        string
	envID      *uuid.UUID
	secret     bool
	before     []byte // empty for create
	after      []byte
}

func writeConfigUpsertAudit(ctx context.Context, hadExisting bool, info configAuditInfo) error {
	verb := "create"
	if hadExisting {
		verb = "update"
	}

	metaIn := configMetadataInput{
		feature: info.feature,
		key:     info.key,
		envID:   info.envID,
		secret:  info.secret,
		after:   info.after,
	}
	if hadExisting {
		metaIn.before = info.before
	}

	return audit.Create(ctx, audit.CreateParams{
		Description: configDescription(verb+"d", info.key, info.secret),
		ObjectType:  info.objectType,
		ObjectID:    info.objectID,
		Metadata:    configMetadata(verb, metaIn),
	})
}

type configMetadataInput struct {
	feature string
	key     string
	envID   *uuid.UUID
	secret  bool
	before  []byte
	after   []byte
}

func configMetadata(verb string, in configMetadataInput) map[string]any {
	m := map[string]any{
		"verb":    verb,
		"feature": in.feature,
		"key":     in.key,
		"secret":  in.secret,
	}
	if in.envID != nil {
		m["envId"] = in.envID.String()
	}
	if in.before != nil {
		m["before"] = configValueForMetadata(in.before, in.secret)
	}
	if in.after != nil {
		m["after"] = configValueForMetadata(in.after, in.secret)
	}
	return m
}

func configValueForMetadata(raw []byte, secret bool) any {
	if secret {
		return "<redacted>"
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	return v
}

func configDescription(verbPast, key string, secret bool) string {
	s := fmt.Sprintf("%s config %s", verbPast, key)
	if secret {
		s += " (secret)"
	}
	return s
}

func stringPtrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
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
