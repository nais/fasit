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
	"text/template"
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
	mv, envKind, err := MappingValuesForEnvironment(ctx, envID, true)
	if err != nil {
		return nil, err
	}

	includeKeys := []string{}
	for key, fv := range f.Values {
		if fv.Config != nil && !slices.Contains(fv.IgnoreKind, envKind) {
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

	data := &helmRenderData{
		mv:         mv,
		envKind:    envKind,
		configVals: vals,
		configMap:  mp,
	}
	return renderHelmValues(data, f, templateFuncs, true)
}

// probeSecretSentinel is the high-entropy placeholder injected into both
// environment and config secret values during a probe render. Any computed
// output that differs between the control and probe renders depends on at
// least one secret input.
const probeSecretSentinel = "__FASIT_PROBE_a9f4e1c8d7b2__" //#nosec G101 -- placeholder, not a credential

// helmRenderData holds everything needed to render helm values without
// additional database access. Fetched once, rendered multiple times for
// taint detection.
type helmRenderData struct {
	mv            *ComputedValues
	envKind       model.EnvironmentKind
	configVals    []featuresql.ConfigForEnvironmentFilteredByKeysRow
	configMap     map[string]any
	secretEnvKeys map[string]bool // set of env value keys marked as secret
}

func fetchHelmRenderData(ctx context.Context, f *model.Feature, envID uuid.UUID) (*helmRenderData, error) {
	mv, envKind, err := MappingValuesForEnvironment(ctx, envID, true)
	if err != nil {
		return nil, err
	}

	includeKeys := []string{}
	for key, fv := range f.Values {
		if fv.Config != nil && !slices.Contains(fv.IgnoreKind, envKind) {
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

	env, err := environment.Get(ctx, envID)
	if err != nil {
		return nil, fmt.Errorf("fetchHelmRenderData: get environment: %w", err)
	}

	secretRows, err := querier(ctx).ListSecretKeysForTenant(ctx, env.TenantID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("fetchHelmRenderData: list secret keys: %w", err)
	}

	secretEnvKeys := make(map[string]bool, len(secretRows))
	for _, row := range secretRows {
		secretEnvKeys[row.Key] = true
	}

	return &helmRenderData{
		mv:            mv,
		envKind:       envKind,
		configVals:    vals,
		configMap:     mp,
		secretEnvKeys: secretEnvKeys,
	}, nil
}

// renderHelmValues renders the helm config map from pre-fetched data.
// When validate is true, missing required fields cause an error.
func renderHelmValues(data *helmRenderData, f *model.Feature, funcs template.FuncMap, validate bool) (map[string]any, error) {
	mp := data.configMap
	mv := data.mv

	if err := GenerateWith(f.Values, data.envKind, mv, mp, funcs); err != nil {
		return nil, err
	}

	mp["fasit"] = map[string]any{
		"tenant": map[string]string{
			"name": mv.Tenant.Name,
		},
		"env": map[string]string{
			"name": mv.Env["name"].(string),
			"kind": data.envKind.String(),
		},
	}

	if validate {
		missing := validateFields(f, data.envKind, data.configVals, mp)
		if len(missing) > 0 {
			return nil, &errs.ErrMissingRequiredFields{Fields: missing}
		}
	}

	return mp, nil
}

func cloneConfigMap(m map[string]any) map[string]any {
	b, _ := json.Marshal(m)
	ret := make(map[string]any)
	_ = json.Unmarshal(b, &ret)
	return ret
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

// maskEnvSecrets replaces secret env values in mv with the probe sentinel.
func maskEnvSecrets(mv *ComputedValues, secretKeys map[string]bool) {
	maskMapKeys(mv.Env, secretKeys)
	maskMapKeys(mv.Management, secretKeys)
	for _, envMap := range mv.Envs {
		maskMapKeys(envMap, secretKeys)
	}
}

func maskMapKeys(m map[string]any, secretKeys map[string]bool) {
	for k := range m {
		if secretKeys[k] {
			m[k] = probeSecretSentinel
		}
	}
}

// cloneComputedValues returns a deep copy of mv so that mutations
// (e.g. sentinel injection) don't affect the original.
func cloneComputedValues(mv *ComputedValues) *ComputedValues {
	clone := &ComputedValues{
		Kind:   mv.Kind,
		Tenant: mv.Tenant,
	}
	clone.Env = cloneStringAnyMap(mv.Env)
	clone.Management = cloneStringAnyMap(mv.Management)
	clone.Envs = make([]map[string]any, len(mv.Envs))
	for i, e := range mv.Envs {
		clone.Envs[i] = cloneStringAnyMap(e)
	}
	return clone
}

func cloneStringAnyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	c := make(map[string]any, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

// HelmValuesWithSecretTaint renders the helm values for f in envID and reports
// the set of computed value keys whose rendered output depends on a secret
// input (a secret config or a secret environment value).
//
// The taint comparison uses deterministic template functions so that
// non-deterministic functions (now, randAlphaNum, …) do not cause false
// positives.
//
// Both env and config secrets are masked with the same high-entropy
// sentinel for the probe render.
//
// If probeOK is false, the caller should treat all computed values as
// potentially secret.
//
// NOTE: non-string secret configs (int, bool, string_array) receive a
// string sentinel which may cause a type mismatch in the probe template
// render. In that case probeOK will be false and the caller falls back
// to pessimistically masking all computed values.
func HelmValuesWithSecretTaint(ctx context.Context, f *model.Feature, envID uuid.UUID) (rendered map[string]any, taint map[string]bool, probeOK bool, err error) {
	data, err := fetchHelmRenderData(ctx, f, envID)
	if err != nil {
		return nil, nil, false, err
	}
	return renderHelmValuesWithSecretTaint(data, f)
}

func renderHelmValuesWithSecretTaint(data *helmRenderData, f *model.Feature) (rendered map[string]any, taint map[string]bool, probeOK bool, err error) {
	// Snapshot the pre-render configMap so control/probe start from the same
	// state as the real render. addToMap is write-once: if the real render's
	// computed values leaked into control/probe, they'd skip re-rendering and
	// the taint diff would always be empty.
	originalConfigMap := cloneConfigMap(data.configMap)

	real, err := renderHelmValues(data, f, templateFuncs, false)
	if err != nil {
		return nil, nil, false, err
	}

	controlData := *data
	controlData.configMap = cloneConfigMap(originalConfigMap)
	control, cerr := renderHelmValues(&controlData, f, deterministicTemplateFuncs, false)

	// Probe render (deterministic funcs, secrets masked with sentinel, no validation).
	probeMV := cloneComputedValues(data.mv)
	maskEnvSecrets(probeMV, data.secretEnvKeys)
	probeCfg := cloneConfigMap(originalConfigMap)
	for _, key := range f.SecretKeys() {
		setNestedSentinel(probeCfg, key)
	}
	probeData := &helmRenderData{
		mv:         probeMV,
		envKind:    data.envKind,
		configVals: data.configVals,
		configMap:  probeCfg,
	}

	probe, perr := renderHelmValues(probeData, f, deterministicTemplateFuncs, false)

	if cerr != nil || perr != nil {
		return real, nil, false, nil
	}

	taint = computedSecretTaint(f.Values, control, probe)
	return real, taint, true, nil
}

// computedSecretTaint reports which computed value keys in values render to
// different output between the control and probe maps. A computed value with a
// differing rendered value depends on at least one secret input.
func computedSecretTaint(values model.Values, control, probe map[string]any) map[string]bool {
	taint := map[string]bool{}
	for key, val := range values {
		if val.Computed == nil {
			continue
		}
		a, aok := lookupNested(control, key)
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

func FeatureByName(ctx context.Context, name string) (*model.Feature, error) {
	f, err := querier(ctx).FeatureByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get feature by name from db: %w", err)
	}

	feature, err := featureFromSQL(f.FeatureDatum)
	if err != nil {
		return nil, err
	}

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
