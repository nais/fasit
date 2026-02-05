package feature

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/errs"
	"github.com/nais/fasit/internal/feature/featuresql"
	"github.com/nais/fasit/internal/graph/model"
)

type ctxKey int

const querierKey ctxKey = iota

func Register(ctx context.Context, pool *pgxpool.Pool) context.Context {
	return context.WithValue(ctx, querierKey, featuresql.New(pool))
}

func querier(ctx context.Context) featuresql.Querier {
	return ctx.Value(querierKey).(featuresql.Querier)
}

func HelmValues(ctx context.Context, f *model.Feature, envID uuid.UUID) (map[string]any, error) {
	mv, envKind, err := MappingValuesForEnvironment(ctx, envID, true)
	if err != nil {
		return nil, err
	}

	includeKeys := []string{}
	for key, f := range f.Values {
		if f.Config != nil && !slices.Contains(f.IgnoreKind, envKind) {
			includeKeys = append(includeKeys, key)
		}
	}

	vals, err := querier(ctx).ConfigForEnvironmentFilteredByKeys(ctx, featuresql.ConfigForEnvironmentFilteredByKeysParams{
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

	err = Generate(f.Values, envKind, mv, mp)

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
		return nil, &errs.ErrMissingRequiredFields{Fields: missing}
	}

	return mp, err
}

func MappingValuesForEnvironment(ctx context.Context, envID uuid.UUID, showSensitive bool) (*ComputedValues, model.EnvironmentKind, error) {
	env, err := environment.Get(ctx, envID)
	if err != nil {
		return nil, "", fmt.Errorf("envValuesForEnv: failed to get environment: %w", err)
	}

	tenant, err := environment.GetTenant(ctx, env.TenantID)
	if err != nil {
		return nil, env.Kind, fmt.Errorf("envValuesForEnv: failed to get tenant: %w", err)
	}
	mv := &ComputedValues{
		Kind: env.Kind,
		Tenant: ComputedTenant{
			Name: tenant.Name,
		},
	}

	values, err := querier(ctx).ListMappingValuesForTenant(ctx, featuresql.ListMappingValuesForTenantParams{
		Tenantid:      tenant.ID,
		Showsensitive: showSensitive,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return mv, env.Kind, nil
		}
		return nil, env.Kind, fmt.Errorf("envValuesForEnv: failed to get environment values: %w", err)
	}

	for _, env := range values {
		val := map[string]any{}
		if err := json.Unmarshal(env.Values, &val); err != nil {
			return nil, model.EnvironmentKind(env.Kind), fmt.Errorf("envValuesForEnv: failed to unmarshal values for %q: %w", env.Name, err)
		}
		val["name"] = env.Name
		val["kind"] = string(env.Kind)

		if env.ID == envID {
			mv.Env = val
		}
		if env.Kind == featuresql.EnvironmentKind(model.EnvironmentKindManagement) {
			mv.Management = val
		} else {
			mv.Envs = append(mv.Envs, val)
		}
	}

	return mv, env.Kind, nil
}

func FeatureDataCreate(ctx context.Context, feat model.Feature, details *FeatureTemplateDetails) error {
	// TODO: Use pgx v5 instead of []byte
	dep, err := json.Marshal(feat.Dependencies)
	if err != nil {
		return fmt.Errorf("marshal dependencies to json: %w", err)
	}
	vals, err := json.Marshal(feat.Values)
	if err != nil {
		return fmt.Errorf("marshal values to json: %w", err)
	}
	defaultVals, err := json.Marshal(feat.ValuesYAML)
	if err != nil {
		return fmt.Errorf("marshal default values to json: %w", err)
	}

	detailsBytes, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("marshal details to json: %w", err)
	}

	rename, err := json.Marshal(feat.Rename)
	if err != nil {
		return fmt.Errorf("marshal rename to json: %w", err)
	}

	return querier(ctx).FeatureDataCreate(ctx, featuresql.FeatureDataCreateParams{
		FeatureName:   feat.Name,
		Version:       feat.Version,
		Chart:         feat.Chart,
		Description:   feat.Description,
		Source:        feat.Source,
		Kinds:         environmentKindToSQL(feat.EnvironmentKinds),
		Dependencies:  dep,
		Values:        vals,
		DefaultValues: defaultVals,
		Timeout:       feat.Timeout.Milliseconds(),
		TplDetails:    detailsBytes,
		Rename:        rename,
	})
}

func FeatureVersionUpdate(ctx context.Context, name string, version string) error {
	return querier(ctx).FeatureVersionUpdate(ctx, featuresql.FeatureVersionUpdateParams{
		Name:    name,
		Version: version,
	})
}
