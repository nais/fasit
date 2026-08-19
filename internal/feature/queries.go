package feature

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"text/template"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/database/types"
	"github.com/nais/fasit/internal/dbtx"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/errs"
	"github.com/nais/fasit/internal/feature/featuresql"
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
	data, err := fetchHelmRenderData(ctx, f, envID)
	if err != nil {
		return nil, err
	}
	return RenderHelmValues(data, f, TemplateFuncs, true)
}

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

	globalConfigs, err := querier(ctx).ListGlobalConfigByFeature(ctx, f.Name)
	if err != nil {
		return nil, err
	}

	envConfigs, err := querier(ctx).ListEnvConfigByFeature(ctx, featuresql.ListEnvConfigByFeatureParams{
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
		Fasit: ComputedFasit{
			IAPAudience: IAPAudience(),
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

func CreateFeatureData(ctx context.Context, feat Feature, details *FeatureTemplateDetails) error {
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

	err = querier(ctx).CreateFeatureData(ctx, featuresql.CreateFeatureDataParams{
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

func FeatureNames(ctx context.Context) ([]string, error) {
	return querier(ctx).FeatureNames(ctx)
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

// LatestDeployInstructionsForEnvironment returns the latest deploy instruction
// per feature in an environment, keyed by feature name. One query instead of
// one per feature.
func LatestDeployInstructionsForEnvironment(ctx context.Context, envID uuid.UUID) (map[string]*DeployInstruction, error) {
	rows, err := querier(ctx).ListLatestDeployInstructionsForEnvironment(ctx, envID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]*DeployInstruction, len(rows))
	for _, di := range rows {
		faID := di.FeatureAssignmentID
		out[di.FeatureName] = &DeployInstruction{
			ID:                  di.ID,
			EnvironmentID:       di.EnvironmentID,
			FeatureAssignmentID: &faID,
			FeatureName:         di.FeatureName,
			FeatureVersion:      di.FeatureVersion,
			Status:              DeployStatus(di.Status),
			Hash:                di.Hash,
			Created:             di.Created,
			LastModified:        di.Created,
		}
	}
	return out, nil
}

// LatestDeployedForEnvironment returns the latest successfully deployed deploy
// instruction per feature in an environment, keyed by feature name. One query
// instead of one per feature.
func LatestDeployedForEnvironment(ctx context.Context, envID uuid.UUID) (map[string]*DeployInstruction, error) {
	rows, err := querier(ctx).ListLatestDeployedForEnvironment(ctx, envID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]*DeployInstruction, len(rows))
	for _, di := range rows {
		faID := di.FeatureAssignmentID
		out[di.FeatureName] = &DeployInstruction{
			ID:                  di.ID,
			EnvironmentID:       di.EnvironmentID,
			FeatureAssignmentID: &faID,
			FeatureName:         di.FeatureName,
			FeatureVersion:      di.FeatureVersion,
			Status:              DeployStatus(di.Status),
			Hash:                di.Hash,
			Created:             di.Created,
			LastModified:        di.Created,
		}
	}
	return out, nil
}

// LatestDeployInstructionsForFeature returns the latest deploy instruction per
// environment for a feature, keyed by environment id. One query instead of one
// per environment.
func LatestDeployInstructionsForFeature(ctx context.Context, featureName string) (map[uuid.UUID]*DeployInstruction, error) {
	rows, err := querier(ctx).ListLatestDeployInstructionsForFeature(ctx, featureName)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]*DeployInstruction, len(rows))
	for _, di := range rows {
		faID := di.FeatureAssignmentID
		out[di.EnvironmentID] = &DeployInstruction{
			ID:                  di.ID,
			EnvironmentID:       di.EnvironmentID,
			FeatureAssignmentID: &faID,
			FeatureName:         di.FeatureName,
			FeatureVersion:      di.FeatureVersion,
			Status:              DeployStatus(di.Status),
			Hash:                di.Hash,
			Created:             di.Created,
			LastModified:        di.Created,
		}
	}
	return out, nil
}

// LatestDeployedForFeature returns the latest successfully deployed deploy
// instruction per environment for a feature, keyed by environment id. One query
// instead of one per environment.
func LatestDeployedForFeature(ctx context.Context, featureName string) (map[uuid.UUID]*DeployInstruction, error) {
	rows, err := querier(ctx).ListLatestDeployedForFeature(ctx, featureName)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]*DeployInstruction, len(rows))
	for _, di := range rows {
		faID := di.FeatureAssignmentID
		out[di.EnvironmentID] = &DeployInstruction{
			ID:                  di.ID,
			EnvironmentID:       di.EnvironmentID,
			FeatureAssignmentID: &faID,
			FeatureName:         di.FeatureName,
			FeatureVersion:      di.FeatureVersion,
			Status:              DeployStatus(di.Status),
			Hash:                di.Hash,
			Created:             di.Created,
			LastModified:        di.Created,
		}
	}
	return out, nil
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
