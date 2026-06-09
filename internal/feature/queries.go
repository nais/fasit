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
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/database/types"
	"github.com/nais/fasit/internal/dbtx"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/errs"
	"github.com/nais/fasit/internal/feature/featuresql"
	"github.com/nais/fasit/internal/feature/featureutil"
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

func helmValues(ctx context.Context, f *Feature, envID uuid.UUID) (map[string]any, error) {
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

	vals := MergeConfigs(globalConfigs, envConfigs, includeKeys)

	mp, err := MakeHelmConfigMap(vals)
	if err != nil {
		return nil, err
	}

	data := &HelmRenderData{
		MV:         mv,
		EnvKind:    envKind,
		ConfigVals: vals,
		ConfigMap:  mp,
	}
	return RenderHelmValues(data, f, TemplateFuncs, true)
}

// probeSecretSentinel is the high-entropy placeholder injected into both
// environment and config secret values during a probe render. Any computed
// output that differs between the control and probe renders depends on at
// least one secret input.
const probeSecretSentinel = "__FASIT_PROBE_a9f4e1c8d7b2__" // #nosec G101 -- placeholder, not a credential

// HelmRenderData holds everything needed to render helm values without
// additional database access. Fetched once, rendered multiple times for
// taint detection.
type HelmRenderData struct {
	MV            *ComputedValues
	EnvKind       environment.EnvironmentKind
	ConfigVals    []MergedConfigRow
	ConfigMap     map[string]any
	SecretEnvKeys map[string]bool
}

func fetchHelmRenderData(ctx context.Context, f *Feature, envID uuid.UUID) (*HelmRenderData, error) {
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

	vals := MergeConfigs(globalConfigs, envConfigs, includeKeys)

	mp, err := MakeHelmConfigMap(vals)
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

	return &HelmRenderData{
		MV:            mv,
		EnvKind:       envKind,
		ConfigVals:    vals,
		ConfigMap:     mp,
		SecretEnvKeys: secretEnvKeys,
	}, nil
}

// RenderHelmValues renders the helm config map from pre-fetched data.
// When validate is true, missing required fields cause an error.
func RenderHelmValues(data *HelmRenderData, f *Feature, funcs template.FuncMap, validate bool) (map[string]any, error) {
	mp := data.ConfigMap
	mv := data.MV

	if err := GenerateWith(f.Values, data.EnvKind, mv, mp, funcs); err != nil {
		return nil, err
	}

	mp["fasit"] = map[string]any{
		"tenant": map[string]string{
			"name": mv.Tenant.Name,
		},
		"env": map[string]string{
			"name": mv.Env["name"].(string),
			"kind": data.EnvKind.String(),
		},
	}

	if validate {
		missing := ValidateFields(f, data.EnvKind, data.ConfigVals, mp)
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
func HelmValuesWithSecretTaint(ctx context.Context, f *Feature, envID uuid.UUID) (rendered map[string]any, taint map[string]bool, probeOK bool, err error) {
	data, err := fetchHelmRenderData(ctx, f, envID)
	if err != nil {
		return nil, nil, false, err
	}
	return renderHelmValuesWithSecretTaint(data, f)
}

func renderHelmValuesWithSecretTaint(data *HelmRenderData, f *Feature) (rendered map[string]any, taint map[string]bool, probeOK bool, err error) {
	// Snapshot the pre-render configMap so control/probe start from the same
	// state as the real render. addToMap is write-once: if the real render's
	// computed values leaked into control/probe, they'd skip re-rendering and
	// the taint diff would always be empty.
	originalConfigMap := cloneConfigMap(data.ConfigMap)

	real, err := RenderHelmValues(data, f, TemplateFuncs, false)
	if err != nil {
		return nil, nil, false, err
	}

	controlData := *data
	controlData.ConfigMap = cloneConfigMap(originalConfigMap)
	control, cerr := RenderHelmValues(&controlData, f, deterministicTemplateFuncs, false)

	// Probe render (deterministic funcs, secrets masked with sentinel, no validation).
	probeMV := cloneComputedValues(data.MV)
	maskEnvSecrets(probeMV, data.SecretEnvKeys)
	probeCfg := cloneConfigMap(originalConfigMap)
	for _, key := range f.SecretKeys() {
		setNestedSentinel(probeCfg, key)
	}
	probeData := &HelmRenderData{
		MV:         probeMV,
		EnvKind:    data.EnvKind,
		ConfigVals: data.ConfigVals,
		ConfigMap:  probeCfg,
	}

	probe, perr := RenderHelmValues(probeData, f, deterministicTemplateFuncs, false)

	if cerr != nil || perr != nil {
		return real, nil, false, nil
	}

	taint = computedSecretTaint(f.Values, control, probe)
	return real, taint, true, nil
}

// computedSecretTaint reports which computed value keys in values render to
// different output between the control and probe maps. A computed value with a
// differing rendered value depends on at least one secret input.
func computedSecretTaint(values Values, control, probe map[string]any) map[string]bool {
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

func HelmValues(ctx context.Context, f *Feature, envID uuid.UUID) (map[string]any, error) {
	return helmValues(ctx, f, envID)
}

func MappingValuesForEnvironment(ctx context.Context, envID uuid.UUID, showSensitive bool) (*ComputedValues, environment.EnvironmentKind, error) {
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
			return nil, environment.EnvironmentKind(env.Kind), fmt.Errorf("envValuesForEnv: failed to unmarshal values for %q: %w", env.Name, err)
		}
		val["name"] = env.Name
		val["kind"] = string(env.Kind)

		if env.ID == envID {
			mv.Env = val
		}
		if env.Kind == types.EnvironmentKind(environment.EnvironmentKindManagement) {
			mv.Management = val
		} else {
			mv.Envs = append(mv.Envs, val)
		}
	}

	return mv, env.Kind, nil
}

func FeatureDataCreate(ctx context.Context, feat Feature, details *FeatureTemplateDetails) error {
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

func FeatureByName(ctx context.Context, name string) (*Feature, error) {
	f, err := querier(ctx).LatestFeatureData(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get latest feature data for %q: %w", name, err)
	}
	return featureFromSQL(f.FeatureDatum)
}

func FeatureByNameVersion(ctx context.Context, name, version string) (*Feature, error) {
	f, err := querier(ctx).FeatureDataByVersion(ctx, featuresql.FeatureDataByVersionParams{
		FeatureName: name,
		Version:     version,
	})
	if err != nil {
		return nil, fmt.Errorf("get feature data for %q version %q: %w", name, version, err)
	}
	return featureFromSQL(f.FeatureDatum)
}

type FeatureVersion struct {
	Version     string
	Description string
	Source      string
	LastUpdated time.Time
}

func FeatureVersions(ctx context.Context, name string) ([]FeatureVersion, error) {
	rows, err := querier(ctx).FeatureVersionRows(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("list versions for %q: %w", name, err)
	}
	ret := make([]FeatureVersion, len(rows))
	for i, row := range rows {
		ret[i] = FeatureVersion{
			Version:     row.Version,
			Description: row.Description,
			Source:      row.Source,
			LastUpdated: asTime(row.LastUpdated),
		}
	}
	return ret, nil
}

func asTime(v any) time.Time {
	if t, ok := v.(time.Time); ok {
		return t
	}
	return time.Time{}
}

type FeatureIndexRow struct {
	Name        string
	Description string
	Source      string
}

func FeatureNames(ctx context.Context) ([]string, error) {
	return querier(ctx).FeatureNames(ctx)
}

func FeatureIndexRows(ctx context.Context) ([]FeatureIndexRow, error) {
	rows, err := querier(ctx).FeatureIndexRows(ctx)
	if err != nil {
		return nil, err
	}

	ret := make([]FeatureIndexRow, len(rows))
	for i, row := range rows {
		ret[i] = FeatureIndexRow{
			Name:        row.Name,
			Description: row.Description,
			Source:      row.Source,
		}
	}
	return ret, nil
}

func GetLatestDeployInstruction(ctx context.Context, envID uuid.UUID, featureName string) (*DeployInstruction, error) {
	di, err := querier(ctx).GetLatestDeployInstruction(ctx, featuresql.GetLatestDeployInstructionParams{
		EnvironmentID: envID,
		FeatureName:   featureName,
	})
	if err != nil {
		return nil, err
	}

	return &DeployInstruction{
		ID:                  di.ID,
		EnvironmentID:       di.EnvironmentID,
		FeatureAssignmentID: &di.FeatureAssignmentID,
		FeatureName:         di.FeatureName,
		FeatureVersion:      di.FeatureVersion,
		Status:              DeployStatus(di.Status),
		Hash:                di.Hash,
		Created:             di.Created,
		LastModified:        di.Created,
	}, nil
}

func GetLatestDeployedDeployInstruction(ctx context.Context, envID uuid.UUID, featureName string) (*DeployInstruction, error) {
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

	return &DeployInstruction{
		ID:                  di.ID,
		EnvironmentID:       di.EnvironmentID,
		FeatureAssignmentID: &di.FeatureAssignmentID,
		FeatureName:         di.FeatureName,
		FeatureVersion:      di.FeatureVersion,
		Status:              DeployStatus(di.Status),
		Hash:                di.Hash,
		Created:             di.Created,
		LastModified:        di.Created,
	}, nil
}

// ListRecentDeployInstructions returns deploy history for an environment x
// feature, one entry per deploy (diid). The deploy_log holds one row per status
// transition, so rows are grouped by diid here: the earliest row is the publish
// (version/hash/created), the latest row carries the current status.
func ListRecentDeployInstructions(ctx context.Context, envID uuid.UUID, featureName string, limit int) ([]*DeployInstruction, error) {
	rows, err := querier(ctx).ListDeployLog(ctx, featuresql.ListDeployLogParams{
		EnvironmentID: envID,
		FeatureName:   featureName,
	})
	if err != nil {
		return nil, err
	}

	byDiid := make(map[uuid.UUID]*DeployInstruction, len(rows))
	order := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows { // ordered by created ASC
		d, ok := byDiid[r.Diid]
		if !ok {
			d = &DeployInstruction{
				ID:                  r.Diid,
				EnvironmentID:       r.EnvironmentID,
				FeatureAssignmentID: &r.FeatureAssignmentID,
				FeatureName:         r.FeatureName,
				FeatureVersion:      r.FeatureVersion,
				Hash:                r.Hash,
				Created:             r.Created,
			}
			byDiid[r.Diid] = d
			order = append(order, r.Diid)
		}
		d.Status = DeployStatus(r.Status)
		d.LastModified = r.Created
	}

	// order holds diids by publish time ascending; emit most recent first.
	result := make([]*DeployInstruction, 0, len(order))
	for i := len(order) - 1; i >= 0; i-- {
		result = append(result, byDiid[order[i]])
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}
