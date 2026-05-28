package reconciler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/reconciler/reconcilersql"
)

type snapshot struct {
	environments   []environment
	deployments    []*reconcileDeployment
	healthByEnv    map[uuid.UUID]time.Time
	disabledByEnv  map[uuid.UUID]map[string]bool
	globalConfig   map[string][]featurepkg.MergedConfigRow
	envConfig      map[uuid.UUID]map[string][]featurepkg.MergedConfigRow
	envValues      map[uuid.UUID]*featurepkg.ComputedValues
	envKinds       map[uuid.UUID]model.EnvironmentKind
	envTenantNames map[uuid.UUID]string
	latestInstr    map[uuid.UUID]map[string]latestInstruction
	deployedFeats  map[uuid.UUID]map[string]bool
}

func (r *Reconciler) fetchSnapshot(ctx context.Context) (*snapshot, error) {
	allEnvRows, err := r.querier.ListAllTenantEnvironments(ctx)
	if err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}

	var envs []environment
	envKinds := make(map[uuid.UUID]model.EnvironmentKind, len(allEnvRows))
	envTenantNames := make(map[uuid.UUID]string, len(allEnvRows))
	for _, row := range allEnvRows {
		envKinds[row.ID] = model.EnvironmentKind(row.Kind)
		envTenantNames[row.ID] = row.TenantName
		if row.Reconcile {
			envs = append(envs, environment{
				ID:         row.ID,
				Name:       row.Name,
				Kind:       model.EnvironmentKind(row.Kind),
				Labels:     map[string]string(row.Labels),
				TenantID:   row.TenantID,
				TenantName: row.TenantName,
			})
		}
	}

	healthRows, err := r.querier.ListHealthStatuses(ctx)
	if err != nil {
		return nil, fmt.Errorf("list health: %w", err)
	}
	healthByEnv := make(map[uuid.UUID]time.Time, len(healthRows))
	for _, h := range healthRows {
		healthByEnv[h.EnvironmentID] = h.ReportedAt.Time
	}

	depRows, err := r.querier.ListLatestDeployments(ctx)
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}
	deployments := make([]*reconcileDeployment, 0, len(depRows))
	for _, row := range depRows {
		dep, err := deploymentFromRow(row)
		if err != nil {
			return nil, err
		}
		deployments = append(deployments, dep)
	}

	disabledRows, err := r.querier.ListDisabledFeatures(ctx)
	if err != nil {
		return nil, fmt.Errorf("list disabled: %w", err)
	}
	disabledByEnv := make(map[uuid.UUID]map[string]bool)
	for _, d := range disabledRows {
		if disabledByEnv[d.EnvironmentID] == nil {
			disabledByEnv[d.EnvironmentID] = make(map[string]bool)
		}
		disabledByEnv[d.EnvironmentID][d.Feature] = true
	}

	globalConfig, err := r.buildGlobalConfigMap(ctx)
	if err != nil {
		return nil, err
	}

	envConfig, err := r.buildEnvConfigMap(ctx)
	if err != nil {
		return nil, err
	}

	envValues, err := r.buildEnvValues(ctx, allEnvRows)
	if err != nil {
		return nil, err
	}

	instrRows, err := r.querier.ListLatestDeployInstructions(ctx)
	if err != nil {
		return nil, fmt.Errorf("list instructions: %w", err)
	}
	latestInstr := make(map[uuid.UUID]map[string]latestInstruction)
	for _, row := range instrRows {
		if latestInstr[row.EnvironmentID] == nil {
			latestInstr[row.EnvironmentID] = make(map[string]latestInstruction)
		}
		latestInstr[row.EnvironmentID][row.FeatureName] = latestInstruction{
			Hash:   row.Hash,
			Status: row.Status,
		}
	}

	deployedRows, err := r.querier.ListDeployedFeatures(ctx)
	if err != nil {
		return nil, fmt.Errorf("list deployed: %w", err)
	}
	deployedFeats := make(map[uuid.UUID]map[string]bool)
	for _, row := range deployedRows {
		if deployedFeats[row.EnvironmentID] == nil {
			deployedFeats[row.EnvironmentID] = make(map[string]bool)
		}
		deployedFeats[row.EnvironmentID][row.FeatureName] = true
	}

	return &snapshot{
		environments:   envs,
		deployments:    deployments,
		healthByEnv:    healthByEnv,
		disabledByEnv:  disabledByEnv,
		globalConfig:   globalConfig,
		envConfig:      envConfig,
		envValues:      envValues,
		envKinds:       envKinds,
		envTenantNames: envTenantNames,
		latestInstr:    latestInstr,
		deployedFeats:  deployedFeats,
	}, nil
}

func (r *Reconciler) buildGlobalConfigMap(ctx context.Context) (map[string][]featurepkg.MergedConfigRow, error) {
	rows, err := r.querier.ListAllGlobalConfigs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list global configs: %w", err)
	}
	m := make(map[string][]featurepkg.MergedConfigRow)
	for _, row := range rows {
		m[row.Feature] = append(m[row.Feature], featurepkg.MergedConfigRow{
			ID:      row.ID,
			Key:     row.Key,
			Value:   row.Value,
			Secret:  row.Secret,
			Created: row.Created,
		})
	}
	return m, nil
}

func (r *Reconciler) buildEnvConfigMap(ctx context.Context) (map[uuid.UUID]map[string][]featurepkg.MergedConfigRow, error) {
	rows, err := r.querier.ListAllEnvConfigs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list env configs: %w", err)
	}
	m := make(map[uuid.UUID]map[string][]featurepkg.MergedConfigRow)
	for _, row := range rows {
		if m[row.EnvironmentID] == nil {
			m[row.EnvironmentID] = make(map[string][]featurepkg.MergedConfigRow)
		}
		eid := row.EnvironmentID
		m[row.EnvironmentID][row.Feature] = append(m[row.EnvironmentID][row.Feature], featurepkg.MergedConfigRow{
			ID:            row.ID,
			Key:           row.Key,
			Value:         row.Value,
			Secret:        row.Secret,
			Created:       row.Created,
			EnvironmentID: &eid,
		})
	}
	return m, nil
}

func (r *Reconciler) buildEnvValues(ctx context.Context, envRows []reconcilersql.ListAllTenantEnvironmentsRow) (map[uuid.UUID]*featurepkg.ComputedValues, error) {
	valueRows, err := r.querier.ListAllEnvironmentValues(ctx)
	if err != nil {
		return nil, fmt.Errorf("list env values: %w", err)
	}

	type envVal struct {
		Key   string
		Value []byte
	}
	byEnv := make(map[uuid.UUID][]envVal)
	for _, v := range valueRows {
		byEnv[v.EnvironmentID] = append(byEnv[v.EnvironmentID], envVal{
			Key:   v.Key,
			Value: v.Value,
		})
	}

	// Build tenant→environments index.
	type envInfo struct {
		ID   uuid.UUID
		Name string
		Kind model.EnvironmentKind
	}
	tenantEnvs := make(map[uuid.UUID][]envInfo)
	tenantNames := make(map[uuid.UUID]string)
	for _, e := range envRows {
		tenantEnvs[e.TenantID] = append(tenantEnvs[e.TenantID], envInfo{
			ID:   e.ID,
			Name: e.Name,
			Kind: model.EnvironmentKind(e.Kind),
		})
		tenantNames[e.TenantID] = e.TenantName
	}

	result := make(map[uuid.UUID]*featurepkg.ComputedValues)

	for tenantID, envInfos := range tenantEnvs {
		// Build the per-env value maps for this tenant.
		type parsedEnv struct {
			info envInfo
			vals map[string]any
		}
		parsed := make([]parsedEnv, 0, len(envInfos))
		for _, ei := range envInfos {
			vals := map[string]any{
				"name": ei.Name,
				"kind": string(ei.Kind),
			}
			for _, v := range byEnv[ei.ID] {
				var val any
				if err := json.Unmarshal(v.Value, &val); err != nil {
					return nil, fmt.Errorf("unmarshal env value %s/%s: %w", ei.Name, v.Key, err)
				}
				vals[v.Key] = val
			}
			parsed = append(parsed, parsedEnv{info: ei, vals: vals})
		}

		// For each env in this tenant, build ComputedValues.
		for _, target := range parsed {
			mv := &featurepkg.ComputedValues{
				Kind: target.info.Kind,
				Tenant: featurepkg.ComputedTenant{
					Name: tenantNames[tenantID],
				},
			}

			// Env is the target environment's own values.
			envVals := make(map[string]any)
			envVals["name"] = target.info.Name
			envVals["kind"] = string(target.info.Kind)
			for _, v := range byEnv[target.info.ID] {
				var val any
				if err := json.Unmarshal(v.Value, &val); err != nil {
					return nil, fmt.Errorf("unmarshal env value: %w", err)
				}
				envVals[v.Key] = val
			}
			mv.Env = envVals

			// Envs and Management include all environments in the tenant.
			for _, other := range parsed {
				if other.info.Kind == model.EnvironmentKindManagement {
					mv.Management = other.vals
				} else {
					mv.Envs = append(mv.Envs, other.vals)
				}
			}

			result[target.info.ID] = mv
		}
	}

	return result, nil
}
