package feature

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/dbtx"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/errs"
	"github.com/nais/fasit/internal/feature/featuresql"
	"github.com/nais/fasit/internal/feature/featureutil"
	"github.com/nais/fasit/internal/graph/model"
)

type ctxKey int

// QuerierKey is exposed for testing to override querier with mocks.
// Avoid usage by e.g. using testcontainers.
const (
	QuerierKey ctxKey = iota
	HelmValuesFuncKey
)

func Register(ctx context.Context, pool *pgxpool.Pool) context.Context {
	return context.WithValue(ctx, QuerierKey, featuresql.New(pool))
}

func querier(ctx context.Context) featuresql.Querier {
	q := ctx.Value(QuerierKey).(featuresql.Querier)
	if tx, ok := dbtx.Tx(ctx); ok {
		if real, ok := q.(*featuresql.Queries); ok {
			return real.WithTx(tx)
		}
	}
	return q
}

func helmValues(ctx context.Context, f *model.Feature, envID uuid.UUID) (map[string]any, error) {
	return computeHelmValues(ctx, f, envID, false)
}

// probeSecretSentinel is the placeholder used when re-rendering helm values
// with secret inputs replaced. Any computed output that depends on a secret
// will differ between the real render and the probe render.
const probeSecretSentinel = "__FASIT_PROBE_SECRET__" //#nosec G101 -- placeholder, not a credential

func computeHelmValues(ctx context.Context, f *model.Feature, envID uuid.UUID, probe bool) (map[string]any, error) {
	mv, envKind, err := MappingValuesForEnvironment(ctx, envID, !probe)
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

	if probe {
		for _, key := range f.SecretKeys() {
			setNestedSentinel(mp, key)
		}
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

	if !probe {
		missing := validateFields(f, envKind, vals, mp)
		if len(missing) > 0 {
			return nil, &errs.ErrMissingRequiredFields{Fields: missing}
		}
	}

	return mp, err
}

func setNestedSentinel(m map[string]any, dottedKey string) {
	keys, err := featureutil.SmartDotSplit(dottedKey)
	if err != nil || len(keys) == 0 {
		return
	}
	parent := m
	for i, k := range keys {
		if i == len(keys)-1 {
			if _, ok := parent[k]; ok {
				parent[k] = json.RawMessage(`"` + probeSecretSentinel + `"`)
			}
			return
		}
		next, ok := parent[k].(map[string]any)
		if !ok {
			return
		}
		parent = next
	}
}

// HelmValuesWithSecretTaint renders the helm values for f in envID and reports
// the set of computed value keys whose rendered output depends on a secret
// input (a secret config or a secret environment value).
//
// If the probe render fails, taint is nil and the caller should treat all
// computed values as potentially secret.
func HelmValuesWithSecretTaint(ctx context.Context, f *model.Feature, envID uuid.UUID) (map[string]any, map[string]bool, error) {
	real, err := HelmValues(ctx, f, envID)
	if err != nil {
		return nil, nil, err
	}
	probe, err := computeHelmValues(ctx, f, envID, true)
	if err != nil {
		return real, nil, nil
	}
	taint := computedSecretTaint(f.Values, real, probe)
	return real, taint, nil
}

// computedSecretTaint reports which computed value keys in values render to
// different output between the real and probe maps. A computed value with a
// differing rendered value depends on at least one secret input.
func computedSecretTaint(values model.Values, real, probe map[string]any) map[string]bool {
	taint := map[string]bool{}
	for key, val := range values {
		if val.Computed == nil {
			continue
		}
		a, aok := lookupNested(real, key)
		b, bok := lookupNested(probe, key)
		if aok != bok || !reflect.DeepEqual(a, b) {
			taint[key] = true
		}
	}
	return taint
}

func lookupNested(m map[string]any, dottedKey string) (any, bool) {
	keys, err := featureutil.SmartDotSplit(dottedKey)
	if err != nil || len(keys) == 0 {
		return nil, false
	}
	var cur any = m
	for _, k := range keys {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = mm[k]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

func HelmValues(ctx context.Context, f *model.Feature, envID uuid.UUID) (map[string]any, error) {
	if ctx.Value(HelmValuesFuncKey) != nil {
		return ctx.Value(HelmValuesFuncKey).(func(ctx context.Context, f *model.Feature, envID uuid.UUID) (map[string]any, error))(ctx, f, envID)
	}
	return helmValues(ctx, f, envID)
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

	feature, err := featureFromSQL(f.FeatureDatum)
	if err != nil {
		return nil, err
	}

	feature.HasDeployments = f.Hasdeployments
	return feature, nil
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

		feature.HasDeployments = f.Hasdeployments
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

func FeatureStatesGet(ctx context.Context, envID uuid.UUID) ([]*model.FeatureState, error) {
	ret := []*model.FeatureState{}
	featureStates, err := querier(ctx).FeatureStatesGet(ctx, envID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ret, nil
		}
		return nil, err
	}

	for _, featureState := range featureStates {
		ret = append(ret, &model.FeatureState{
			ID:           model.FeatureStateID(envID, featureState.Name),
			FeatureName:  featureState.Name,
			EnabledAt:    nullTimeToPtr(featureState.EnabledAt),
			Enabled:      featureState.Enabled,
			Created:      featureState.Created.Time,
			LastModified: featureState.LastModified.Time,
			EnvID:        envID,
		})
	}

	return ret, nil
}

func FeatureStatesCreateOrUpdate(ctx context.Context, envID uuid.UUID, feature *model.Feature, enabled bool) (*model.FeatureState, error) {
	if len(feature.Dependencies) > 0 && enabled {
		states, err := FeatureStatesGet(ctx, envID)
		if err != nil {
			return nil, err
		}

		enabledFeatures := []string{}
		for _, state := range states {
			if state.Enabled {
				enabledFeatures = append(enabledFeatures, state.FeatureName)
			}
		}

		missingFeatures := feature.Dependencies.FindMissing(enabledFeatures)
		if len(missingFeatures) > 0 {
			return nil, fmt.Errorf("dependency '%v' not enabled", missingFeatures)
		}
	}

	res, err := querier(ctx).FeatureStateCreateOrUpdate(ctx, featuresql.FeatureStateCreateOrUpdateParams{
		EnvironmentID: envID,
		Feature:       feature.Name,
		Enabled:       enabled,
		Enabledat: pgtype.Timestamptz{
			Time:  time.Now(),
			Valid: enabled,
		},
	})
	if err != nil {
		return nil, err
	}

	msg := fmt.Sprintf("enabled %v", feature.Name)
	if !enabled {
		msg = fmt.Sprintf("disabled %v", feature.Name)
	}

	audit.CreateAudit(ctx, msg, "feature_states", envID.String()+":"+feature.Name)

	return featureStateFromSQL(res), nil
}

func FeatureStateGet(ctx context.Context, envID uuid.UUID, featureName string) (*model.FeatureState, error) {
	featureState, err := querier(ctx).FeatureStateGet(ctx, featuresql.FeatureStateGetParams{
		EnvironmentID: envID,
		Feature:       featureName,
	})

	if err == nil {
		fs := featureStateFromSQL(featureState)
		fs.EnvID = envID
		return fs, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	fs := &model.FeatureState{
		ID:          model.FeatureStateID(envID, featureName),
		FeatureName: featureName,
		EnvID:       envID,
		Enabled:     false,
	}

	env, err := environment.Get(ctx, envID)
	if err != nil {
		return nil, err
	}

	defaultFeatures, err := querier(ctx).AutoInstallNamesForKind(ctx, featuresql.EnvironmentKind(env.Kind.String()))
	if err != nil {
		return nil, err
	}

	if slices.Contains(defaultFeatures, featureName) {
		fs.Enabled = true
		return fs, nil
	}

	hasDeployment, err := querier(ctx).HasMatchingDeployment(ctx, featuresql.HasMatchingDeploymentParams{
		EnvironmentID: envID,
		FeatureName:   featureName,
	})
	if err != nil {
		return nil, err
	}
	if hasDeployment {
		fs.Enabled = true
		return fs, nil
	}

	return fs, nil
}

func RolloutStatesGet(ctx context.Context, envID uuid.UUID) ([]*model.FeatureState, error) {
	ret := []*model.FeatureState{}
	featureStates, err := querier(ctx).RolloutStatesGet(ctx, envID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ret, nil
		}
		return nil, err
	}

	for _, featureState := range featureStates {
		ret = append(ret, &model.FeatureState{
			ID:           model.FeatureStateID(envID, featureState.FeatureName),
			FeatureName:  featureState.FeatureName,
			EnabledAt:    nullTimeToPtr(featureState.EnabledAt),
			Enabled:      featureState.Enabled,
			Created:      featureState.Created.Time,
			LastModified: featureState.LastModified.Time,
			EnvID:        envID,
		})
	}
	return ret, nil
}
