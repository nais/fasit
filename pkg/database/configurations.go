package database

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/feature/featureutil"
	feature "github.com/nais/fasit/pkg/feature2"
	"github.com/nais/fasit/pkg/graph/model"
)

type ConfigRepo interface {
	ConfigCreate(ctx context.Context, c model.NewConfiguration) (*model.Configuration, error)
	ConfigDelete(ctx context.Context, id uuid.UUID) error
	ConfigGet(ctx context.Context, feature string) ([]*model.Configuration, error)
	ConfigGetByID(ctx context.Context, id uuid.UUID) (*model.Configuration, error)
	ConfigGetForEnv(ctx context.Context, feature string, envID uuid.UUID) ([]*model.Configuration, error)
	ConfigOverridesByFeature(ctx context.Context, featureName string) ([]*model.ConfigOverride, error)
	ConfigUpdate(ctx context.Context, id uuid.UUID, c model.UpdateConfiguration) (*model.Configuration, error)
	EnvConfig(ctx context.Context, feature string, envID uuid.UUID) ([]*model.Configuration, error)
	HelmValues(ctx context.Context, feature *model.Feature, envID uuid.UUID) (values map[string]any, err error)
}

func environmentConfigurationFromSQL(c gensql.ConfigurationsEnvironment) *model.Configuration {
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

func globalConfigFromSQL(c gensql.ConfigurationsGlobal) *model.Configuration {
	return &model.Configuration{
		ID:        c.ID,
		GraphVars: model.ConfigurationGraphVars{FeatureName: c.Feature},
		// Description: nullStringToPtr(c.Description),
		Key:     c.Key,
		Content: c.Value,
		Created: c.Created.Time,
		Source:  model.ConfigSourceGlobal,
	}
}

func (r *repo) EnvConfig(ctx context.Context, feature string, envID uuid.UUID) ([]*model.Configuration, error) {
	config, err := r.querier.EnvConfig(ctx, gensql.EnvConfigParams{
		Feature:       feature,
		EnvironmentID: envID,
	})
	if err != nil {
		return nil, err
	}
	retVal := []*model.Configuration{}
	for _, conf := range config {
		retVal = append(retVal, envConfigFromSQL(conf))
	}

	return retVal, nil
}

func envConfigFromSQL(conf gensql.EnvConfigRow) *model.Configuration {
	ret := &model.Configuration{
		ID: conf.ID,
		GraphVars: model.ConfigurationGraphVars{
			EnvironmentID: nullUUIDToPtr(conf.EnvironmentID),
			FeatureName:   conf.Feature,
		},
		// Description:   nullStringToPtr(conf.Description),
		Key:     conf.Key,
		Content: conf.Value,
		Source:  model.ConfigSourceGlobal,
	}

	if conf.EnvironmentID.Valid {
		ret.Source = model.ConfigSourceEnv
		ret.GraphVars.EnvironmentID = nullUUIDToPtr(conf.EnvironmentID)
	}

	return ret
}

func (r *repo) ConfigGet(ctx context.Context, feature string) ([]*model.Configuration, error) {
	config, err := r.querier.ConfigGet(ctx, feature)
	if err != nil {
		return nil, err
	}
	retVal := []*model.Configuration{}
	for _, conf := range config {
		retVal = append(retVal, globalConfigFromSQL(conf))
	}

	return retVal, nil
}

func (r *repo) ConfigGetForEnv(ctx context.Context, feature string, envID uuid.UUID) ([]*model.Configuration, error) {
	params := gensql.ConfigGetForEnvParams{
		Feature:       feature,
		EnvironmentID: envID,
	}
	config, err := r.querier.ConfigGetForEnv(ctx, params)
	if err != nil {
		return nil, err
	}

	retVal := []*model.Configuration{}
	for _, conf := range config {
		retVal = append(retVal, environmentConfigurationFromSQL(conf))
	}

	return retVal, nil
}

func (r *repo) ConfigCreate(ctx context.Context, c model.NewConfiguration) (*model.Configuration, error) {
	value, err := json.Marshal(c.Value)
	if err != nil {
		return nil, err
	}

	if c.EnvironmentID != nil && *c.EnvironmentID != uuid.Nil {
		return r.configEnvCreate(ctx, c, value)
	}

	return r.configGlobalCreate(ctx, c, value)
}

func (r *repo) configEnvCreate(ctx context.Context, c model.NewConfiguration, value []byte) (*model.Configuration, error) {
	config, err := r.querier.ConfigEnvUpdateOrCreate(ctx, gensql.ConfigEnvUpdateOrCreateParams{
		EnvironmentID: *c.EnvironmentID,
		Feature:       c.Feature,
		Description:   ptrToNullString(c.Description),
		Secret:        c.Secret,
		Key:           c.Key,
		Value:         value,
	})
	if err != nil {
		return nil, err
	}

	r.createAudit(ctx, "config created or updated", "configurations_environment", config.ID.String())

	return environmentConfigurationFromSQL(config), nil
}

func (r *repo) configGlobalCreate(ctx context.Context, c model.NewConfiguration, value []byte) (*model.Configuration, error) {
	config, err := r.querier.ConfigGlobalUpdateOrCreate(ctx, gensql.ConfigGlobalUpdateOrCreateParams{
		Feature:     c.Feature,
		Description: ptrToNullString(c.Description),
		Secret:      c.Secret,
		Key:         c.Key,
		Value:       value,
	})
	if err != nil {
		return nil, err
	}

	r.createAudit(ctx, "config created or updated", "configurations_global", config.ID.String())

	return globalConfigFromSQL(config), nil
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

	r.createAudit(ctx, "config updated", "configurations_global", conf.ID.String())

	return globalConfigFromSQL(conf), nil
}

func (r *repo) ConfigDelete(ctx context.Context, id uuid.UUID) error {
	if err := r.querier.ConfigDelete(ctx, id); err != nil {
		return err
	}

	r.createAudit(ctx, "config deleted", "configurations_global", id.String())
	return nil
}

func (r *repo) HelmValues(ctx context.Context, f *model.Feature, envID uuid.UUID) (map[string]any, error) {
	mv, envKind, err := r.MappingValuesForEnvironment(ctx, envID, true)
	if err != nil {
		return nil, err
	}

	includeKeys := []string{}
	for key, f := range f.Values {
		if f.Config != nil && !contains(f.IgnoreKind, envKind) {
			includeKeys = append(includeKeys, key)
		}
	}

	vals, err := r.querier.EnvConfigOnlyKnown(ctx, gensql.EnvConfigOnlyKnownParams{
		Feature:       f.Name,
		EnvironmentID: envID,
		Includedkeys:  includeKeys,
	})
	if err != nil {
		return nil, err
	}

	mp, err := makeHelmConfigMap(vals)
	if err != nil {
		return nil, err
	}

	err = feature.Generate(f.Values, envKind, mv, mp)

	mp["fasit"] = map[string]any{
		"tenant": map[string]string{
			"name": mv.Tenant.Name,
		},
		"env": map[string]string{
			"name": mv.Env["name"].(string),
			"kind": envKind.String(),
		},
	}

	missing := validateFields(f, envKind, vals, mp)
	if len(missing) > 0 {
		return nil, &ErrMissingRequiredFields{Fields: missing}
	}

	return mp, err
}

func validateFields(f *model.Feature, envKind model.EnvironmentKind, values []gensql.EnvConfigOnlyKnownRow, mp map[string]any) []string {
	requiredFields := f.RequiredFields(envKind)

	fields := map[string]bool{}
	for _, req := range requiredFields {
		fields[req] = false
		for _, k := range values {
			if k.Key == req {
				fields[req] = true
			}
		}
	}

	var missing []string
	for field, present := range fields {
		if present {
			continue
		}

		parts, _ := featureutil.SmartDotSplit(field)
		parent := mp
		for _, part := range parts {
			if p, ok := parent[part].(map[string]any); ok {
				parent = p
				continue
			}
			if _, ok := parent[part]; ok {
				continue
			}
			missing = append(missing, field)
			break
		}
	}
	return missing
}

func makeHelmConfigMap(vals []gensql.EnvConfigOnlyKnownRow) (map[string]any, error) {
	val := make(map[string]any)

	for _, v := range vals {
		keys, err := featureutil.SmartDotSplit(v.Key)
		if err != nil {
			return nil, err
		}
		parent := val
		for index, key := range keys {
			if index == len(keys)-1 {
				parent[key] = json.RawMessage(v.Value)
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

func (r *repo) ConfigOverridesByFeature(ctx context.Context, featureName string) ([]*model.ConfigOverride, error) {
	overrides, err := r.querier.ConfigOverridesByFeature(ctx, featureName)
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

func (r *repo) ConfigGetByID(ctx context.Context, id uuid.UUID) (*model.Configuration, error) {
	config, err := r.querier.ConfigGetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return globalConfigFromSQL(config), nil
}

func contains[T comparable](s []T, e T) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
