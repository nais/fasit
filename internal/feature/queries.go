package feature

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"text/template"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/database/types"
	"github.com/nais/fasit/internal/dbtx"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/errs"
	"github.com/nais/fasit/internal/feature/featuresql"
	"github.com/nais/fasit/internal/feature/featureutil"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nsf/jsondiff"
)

type ctxKey int

// QuerierKey is exposed so tests can inject fake queriers on the context.
const (
	QuerierKey ctxKey = iota
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

	globalConfigs, err := querier(ctx).ConfigGlobalListByFeature(ctx, f.Name)
	if err != nil {
		return nil, err
	}

	envConfigs, err := querier(ctx).ConfigEnvListByFeature(ctx, featuresql.ConfigEnvListByFeatureParams{
		Feature:       f.Name,
		EnvironmentID: envID,
	})
	if err != nil {
		return nil, err
	}

	vals := mergeConfigs(globalConfigs, envConfigs, includeKeys)

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
const probeSecretSentinel = "__FASIT_PROBE_a9f4e1c8d7b2__" // #nosec G101 -- placeholder, not a credential

// helmRenderData holds everything needed to render helm values without
// additional database access. Fetched once, rendered multiple times for
// taint detection.
type helmRenderData struct {
	mv            *ComputedValues
	envKind       model.EnvironmentKind
	configVals    []mergedConfigRow
	configMap     map[string]any
	secretEnvKeys map[string]bool
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

	globalConfigs, err := querier(ctx).ConfigGlobalListByFeature(ctx, f.Name)
	if err != nil {
		return nil, err
	}

	envConfigs, err := querier(ctx).ConfigEnvListByFeature(ctx, featuresql.ConfigEnvListByFeatureParams{
		Feature:       f.Name,
		EnvironmentID: envID,
	})
	if err != nil {
		return nil, err
	}

	vals := mergeConfigs(globalConfigs, envConfigs, includeKeys)

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
	maps.Copy(c, m)
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
		if env.Kind == types.EnvironmentKind(model.EnvironmentKindManagement) {
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

	err = querier(ctx).FeatureDataCreate(ctx, featuresql.FeatureDataCreateParams{
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
	})
	if err != nil {
		return err
	}

	return nil
}

func FeatureByName(ctx context.Context, name string) (*model.Feature, error) {
	f, err := querier(ctx).LatestFeatureData(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get latest feature data for %q: %w", name, err)
	}
	return featureFromSQL(f.FeatureDatum)
}

func FeatureNames(ctx context.Context) ([]string, error) {
	return querier(ctx).FeatureNames(ctx)
}

func HelmValueDiff(ctx context.Context, di *model.DeployInstruction, secretKeys []string) (*model.HelmValueDiff, error) {
	ret := &model.HelmValueDiff{
		Difference: model.HelmValueDifferenceNoMatch,
	}

	prev, err := querier(ctx).GetPreviousDeployInstruction(ctx, di.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ret, nil
		}
		return nil, fmt.Errorf("failed to get previous deploy instruction: %w", err)
	}

	currentValues := scrubSecrets(di.Values, secretKeys)
	previousValues := scrubSecrets(prev.Values, secretKeys)

	opts := jsondiff.DefaultHTMLOptions()
	opts.Indent = "\t"
	opts.PrintTypes = true
	opts.SkipMatches = true
	diff, diff2 := jsondiff.Compare(previousValues, currentValues, &opts)
	ret.Diff = diff2

	switch diff {
	case jsondiff.FullMatch:
		ret.Difference = model.HelmValueDifferenceFullMatch
	case jsondiff.SupersetMatch:
		ret.Difference = model.HelmValueDifferenceSupersetMatch
	case jsondiff.BothArgsAreInvalidJson, jsondiff.FirstArgIsInvalidJson, jsondiff.SecondArgIsInvalidJson:
		ret.Difference = model.HelmValueDifferenceInvalidJSON
	}

	return ret, nil
}

func GetLatestDeployInstruction(ctx context.Context, envID uuid.UUID, featureName string) (*model.DeployInstruction, error) {
	di, err := querier(ctx).GetLatestDeployInstruction(ctx, featuresql.GetLatestDeployInstructionParams{
		EnvironmentID: envID,
		FeatureName:   featureName,
	})
	if err != nil {
		return nil, err
	}

	return &model.DeployInstruction{
		ID:             di.ID,
		EnvironmentID:  di.EnvironmentID,
		DeploymentID:   di.DeploymentID,
		FeatureName:    di.FeatureName,
		FeatureVersion: di.FeatureVersion,
		Status:         model.RolloutStatus(di.Status),
		Hash:           di.Hash,
		Created:        di.Created.Time,
		LastModified:   di.LastModified.Time,
		Values:         di.Values,
	}, nil
}

func GetLatestDeployedDeployInstruction(ctx context.Context, envID uuid.UUID, featureName string) (*model.DeployInstruction, error) {
	di, err := querier(ctx).GetLatestDeployedDeployInstruction(ctx, featuresql.GetLatestDeployedDeployInstructionParams{
		EnvironmentID: envID,
		FeatureName:   featureName,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &model.DeployInstruction{
		ID:             di.ID,
		EnvironmentID:  di.EnvironmentID,
		DeploymentID:   di.DeploymentID,
		FeatureName:    di.FeatureName,
		FeatureVersion: di.FeatureVersion,
		Status:         model.RolloutStatus(di.Status),
		Hash:           di.Hash,
		Created:        di.Created.Time,
		LastModified:   di.LastModified.Time,
		Values:         di.Values,
	}, nil
}

func scrubSecrets(data []byte, secretKeys []string) []byte {
	if len(secretKeys) == 0 {
		return data
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return data
	}
	for _, key := range secretKeys {
		parts, err := featureutil.SmartDotSplit(key)
		if err != nil {
			continue
		}
		scrubPath(obj, parts)
	}
	result, err := json.Marshal(obj)
	if err != nil {
		return data
	}
	return result
}

func scrubPath(obj map[string]any, parts []string) {
	for i, part := range parts {
		if i == len(parts)-1 {
			if _, ok := obj[part]; ok {
				obj[part] = "••••••••"
			}
			return
		}
		next, ok := obj[part].(map[string]any)
		if !ok {
			return
		}
		obj = next
	}
}
