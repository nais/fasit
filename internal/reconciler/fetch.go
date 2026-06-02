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
	"golang.org/x/sync/errgroup"
)

type snapshot struct {
	environments   []environment
	assignments    []*reconcileAssignment
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
	// Environments must be fetched first because buildEnvValues needs the rows.
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

	snap := &snapshot{
		environments:   envs,
		envKinds:       envKinds,
		envTenantNames: envTenantNames,
	}

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		rows, err := r.querier.ListHealthStatuses(gctx)
		if err != nil {
			return fmt.Errorf("list health: %w", err)
		}
		m := make(map[uuid.UUID]time.Time, len(rows))
		for _, h := range rows {
			m[h.EnvironmentID] = h.ReportedAt.Time
		}
		snap.healthByEnv = m
		return nil
	})

	g.Go(func() error {
		rows, err := r.querier.ListLatestFeatureAssignments(gctx)
		if err != nil {
			return fmt.Errorf("list assignments: %w", err)
		}
		deps := make([]*reconcileAssignment, 0, len(rows))
		for _, row := range rows {
			dep, err := assignmentFromRow(row)
			if err != nil {
				return err
			}
			deps = append(deps, dep)
		}
		snap.assignments = deps
		return nil
	})

	g.Go(func() error {
		rows, err := r.querier.ListDisabledFeatures(gctx)
		if err != nil {
			return fmt.Errorf("list disabled: %w", err)
		}
		m := make(map[uuid.UUID]map[string]bool)
		for _, d := range rows {
			if m[d.EnvironmentID] == nil {
				m[d.EnvironmentID] = make(map[string]bool)
			}
			m[d.EnvironmentID][d.Feature] = true
		}
		snap.disabledByEnv = m
		return nil
	})

	g.Go(func() error {
		m, err := r.buildGlobalConfigMap(gctx)
		if err != nil {
			return err
		}
		snap.globalConfig = m
		return nil
	})

	g.Go(func() error {
		m, err := r.buildEnvConfigMap(gctx)
		if err != nil {
			return err
		}
		snap.envConfig = m
		return nil
	})

	g.Go(func() error {
		m, err := r.buildEnvValues(gctx, allEnvRows)
		if err != nil {
			return err
		}
		snap.envValues = m
		return nil
	})

	g.Go(func() error {
		rows, err := r.querier.ListLatestDeployInstructions(gctx)
		if err != nil {
			return fmt.Errorf("list instructions: %w", err)
		}
		m := make(map[uuid.UUID]map[string]latestInstruction)
		for _, row := range rows {
			if m[row.EnvironmentID] == nil {
				m[row.EnvironmentID] = make(map[string]latestInstruction)
			}
			m[row.EnvironmentID][row.FeatureName] = latestInstruction{
				Hash:   row.Hash,
				Status: row.Status,
			}
		}
		snap.latestInstr = m
		return nil
	})

	g.Go(func() error {
		rows, err := r.querier.ListDeployedFeatures(gctx)
		if err != nil {
			return fmt.Errorf("list deployed: %w", err)
		}
		m := make(map[uuid.UUID]map[string]bool)
		for _, row := range rows {
			if m[row.EnvironmentID] == nil {
				m[row.EnvironmentID] = make(map[string]bool)
			}
			m[row.EnvironmentID][row.FeatureName] = true
		}
		snap.deployedFeats = m
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return snap, nil
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
