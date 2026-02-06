package feature

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/errs"
	"github.com/nais/fasit/internal/feature/featuresql"
	"github.com/nais/fasit/internal/graph/model"
)

type ctxKey int

// QuerierKey is exposed for testing to override querier with mocks.
// Avoid usage by e.g. using testcontainers.
const QuerierKey ctxKey = iota

func Register(ctx context.Context, pool *pgxpool.Pool) context.Context {
	return context.WithValue(ctx, QuerierKey, featuresql.New(pool))
}

func querier(ctx context.Context) featuresql.Querier {
	return ctx.Value(QuerierKey).(featuresql.Querier)
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

// TODO: rename to FeatureFromRollout or something similar
func RolloutByName(ctx context.Context, name string) (*model.Feature, error) {
	f, err := querier(ctx).RolloutByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get rollout by name from db: %w", err)
	}

	feature, err := featureFromSQL(f.FeatureDatum)
	if err != nil {
		return nil, fmt.Errorf("make feature: %w", err)
	}
	feature.GraphVars = struct {
		EnvironmentID uuid.UUID
		RolloutID     uuid.UUID
	}{
		RolloutID: f.ID,
	}

	return feature, nil
}

func FeatureByName(ctx context.Context, name string) (*model.Feature, error) {
	f, err := querier(ctx).FeatureByName(ctx, name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r, err := RolloutByName(ctx, name)
			if err == nil {
				return r, nil
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf("get rollout by name from db: %w", err)
			}

		}

		return nil, fmt.Errorf("get feature by name from db: %w", err)
	}

	return featureFromSQL(f.FeatureDatum)
}

// TODO: rename function as it is not by env, but by rollout if ci
// TODO: enviromentfeatures are a deployment concept, should be moved to deployment repo, but then we need to refactor repos first
func FeatureByNameForEnv(ctx context.Context, name string, envID uuid.UUID) (*model.Feature, error) {
	feat, err := GetEnvironmentFeature(ctx, envID, name)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("get environment feature from db: %w", err)
	}

	if feat != nil {
		return feat, nil
	}

	env, err := environment.Get(ctx, envID)
	if err != nil {
		return nil, fmt.Errorf("get environment from db: %w", err)
	}

	if env.CI {
		roll, err := RolloutByName(ctx, name)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("get rollout by name from db: %w", err)
		}

		if roll != nil {
			return roll, nil
		}
	}

	return FeatureByName(ctx, name)
}

func GetEnvironmentFeature(ctx context.Context, environmentID uuid.UUID, featureName string) (*model.Feature, error) {
	f, err := querier(ctx).GetEnvironmentFeature(ctx, featuresql.GetEnvironmentFeatureParams{
		EnvironmentID: environmentID,
		FeatureName:   featureName,
	})
	if err != nil {
		return nil, err
	}

	feature, err := featureFromSQL(f.FeatureDatum)
	if err != nil {
		return nil, fmt.Errorf("make feature: %w", err)
	}
	feature.HasDeployments = true

	return feature, nil
}

func Features(ctx context.Context) ([]*model.Feature, error) {
	features, err := querier(ctx).Features(ctx)
	if err != nil {
		return nil, fmt.Errorf("get features from db: %w", err)
	}

	var ret []*model.Feature
	for _, f := range features {
		feature, err := featureFromSQL(f.FeatureDatum)
		if err != nil {
			return nil, fmt.Errorf("make feature: %w", err)
		}

		ret = append(ret, feature)
	}

	return ret, nil
}

func FeaturesForKind(ctx context.Context, kind model.EnvironmentKind, ci bool) ([]*model.Feature, error) {
	features, err := querier(ctx).FeaturesForKind(ctx, kind.String())
	if err != nil {
		return nil, err
	}

	if !ci {
		ret, err := featuresFromSQL(features)
		if err != nil {
			return nil, err
		}

		return ret, nil
	}

	rollouts, err := querier(ctx).RolloutsForKind(ctx, featuresql.EnvironmentKind(kind))
	if err != nil {
		return nil, err
	}

	for _, ro := range rollouts {
		for i, f := range features {
			if f.FeatureDatum.Name == ro.FeatureDatum.Name {
				// delete feature from slice
				features = append(features[:i], features[i+1:]...)
				break
			}
		}
	}

	for _, ro := range rollouts {
		features = append(features, featuresql.FeaturesForKindRow{
			FeatureDatum:   ro.FeatureDatum,
			Hasdeployments: ro.Hasdeployments,
		})
	}

	sort.Slice(features, func(i, j int) bool {
		return features[i].FeatureDatum.Name < features[j].FeatureDatum.Name
	})

	return featuresFromSQL(features)
}
