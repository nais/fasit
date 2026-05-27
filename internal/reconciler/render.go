package reconciler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/errs"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
)

func (r *Reconciler) renderAll(snap *snapshot) []Result {
	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		results []Result
	)

	for _, env := range snap.environments {
		reportedAt, ok := snap.healthByEnv[env.ID]
		if !ok || time.Since(reportedAt) > 3*time.Minute {
			continue
		}

		matched := matchDeployments(snap.deployments, env)
		winners := mostSpecificPerFeature(matched, env)

		for _, dep := range winners {
			if snap.disabledByEnv[env.ID][dep.Feature.Name] {
				mu.Lock()
				results = append(results, Result{
					EnvironmentID:   env.ID,
					EnvironmentName: env.Name,
					TenantName:      env.TenantName,
					DeploymentID:    dep.ID,
					Feature:         dep.Feature,
					Action:          ActionSkipDisabled,
					Message:         "feature reconcile disabled",
				})
				mu.Unlock()
				continue
			}

			wg.Add(1)
			go func(env environment, dep *reconcileDeployment) {
				defer wg.Done()
				res := r.renderOne(snap, env, dep)
				mu.Lock()
				results = append(results, res)
				mu.Unlock()
			}(env, dep)
		}
	}

	wg.Wait()
	return results
}

func (r *Reconciler) renderOne(snap *snapshot, env environment, dep *reconcileDeployment) Result {
	base := Result{
		EnvironmentID:   env.ID,
		EnvironmentName: env.Name,
		TenantName:      env.TenantName,
		DeploymentID:    dep.ID,
		Feature:         dep.Feature,
	}

	// Check dependencies.
	if ok, missing := checkDependencies(dep.Feature, env.ID, snap.deployedFeats); !ok {
		base.Action = ActionFailMissingDeps
		base.Message = "missing dependencies: " + strings.Join(missing, ", ")
		return base
	}

	// Merge configs.
	includeKeys := configIncludeKeys(dep.Feature, env.Kind)
	globalRows := snap.globalConfig[dep.Feature.Name]
	envRows := envConfigForFeature(snap.envConfig, env.ID, dep.Feature.Name)
	merged := mergeConfigRows(globalRows, envRows, includeKeys)

	configMap, err := featurepkg.MakeHelmConfigMap(merged)
	if err != nil {
		base.Action = ActionFailRender
		base.Message = fmt.Sprintf("config map error: %s", err)
		return base
	}

	mv := snap.envValues[env.ID]
	if mv == nil {
		base.Action = ActionFailRender
		base.Message = "no environment values"
		return base
	}
	// Clone mv so parallel goroutines don't race on shared fields
	// (GenerateWith sets mv.Configs during template rendering).
	mvCopy := *mv

	data := &featurepkg.HelmRenderData{
		MV:         &mvCopy,
		EnvKind:    env.Kind,
		ConfigVals: merged,
		ConfigMap:  configMap,
	}

	values, err := featurepkg.RenderHelmValues(data, dep.Feature, featurepkg.TemplateFuncs, true)
	if err != nil {
		var fer *errs.ErrMissingRequiredFields
		if isErrMissingFields(err, &fer) {
			base.Action = ActionFailMissingConfig
			base.Message = fmt.Sprintf("missing required chart config: %s", strings.Join(fer.Fields, ", "))
			return base
		}
		base.Action = ActionFailRender
		base.Message = fmt.Sprintf("render error: %s", err)
		return base
	}

	hash, err := generateHash(values, dep.Feature)
	if err != nil {
		base.Action = ActionFailRender
		base.Message = fmt.Sprintf("hash error: %s", err)
		return base
	}

	base.Values = values
	base.Hash = hash

	// Check against latest instruction.
	if instr, ok := snap.latestInstr[env.ID][dep.Feature.Name]; ok {
		if instr.Status == model.RolloutStatusCreated.String() || instr.Status == model.RolloutStatusPending.String() {
			base.Action = ActionSkipInProgress
			base.Status = instr.Status
			base.Message = "deployment is already in progress"
			return base
		}
		if instr.Hash == hash {
			if instr.Status != model.RolloutStatusFailed.String() {
				base.Action = ActionSkipUnchanged
				base.Status = instr.Status
				base.Message = "no changes in feature"
				return base
			}
		}
	}

	base.Action = ActionDeploy
	base.Message = "deployment instruction sent to naisd"
	return base
}

func matchDeployments(all []*reconcileDeployment, env environment) []*reconcileDeployment {
	var matched []*reconcileDeployment
	for _, dep := range all {
		if labelsContain(env.Labels, dep.TargetLabels) {
			matched = append(matched, dep)
		}
	}
	return matched
}

func mostSpecificPerFeature(deps []*reconcileDeployment, env environment) []*reconcileDeployment {
	winners := map[string]*reconcileDeployment{}
	for _, dep := range deps {
		existing, ok := winners[dep.Feature.Name]
		if !ok || deployment.IsMoreSpecific(dep.TargetLabels, existing.TargetLabels, dep.Created, existing.Created) {
			winners[dep.Feature.Name] = dep
		}
	}

	result := make([]*reconcileDeployment, 0, len(winners))
	for _, d := range winners {
		result = append(result, d)
	}
	slices.SortStableFunc(result, func(a, b *reconcileDeployment) int {
		return a.Created.Compare(b.Created)
	})
	return result
}

func checkDependencies(f *model.Feature, envID uuid.UUID, deployedFeats map[uuid.UUID]map[string]bool) (bool, []string) {
	if len(f.Dependencies) == 0 {
		return true, nil
	}

	deployed := deployedFeats[envID]

	for _, dep := range f.Dependencies {
		if len(dep.AllOf) > 0 {
			var missing []string
			for _, name := range dep.AllOf {
				if !deployed[name] {
					missing = append(missing, name)
				}
			}
			if len(missing) > 0 {
				return false, missing
			}
		}
		if len(dep.AnyOf) > 0 {
			found := false
			for _, name := range dep.AnyOf {
				if deployed[name] {
					found = true
					break
				}
			}
			if !found {
				return false, dep.AnyOf
			}
		}
	}
	return true, nil
}

func configIncludeKeys(f *model.Feature, envKind model.EnvironmentKind) []string {
	var keys []string
	for key, val := range f.Values {
		if val.Config != nil && !slices.Contains(val.IgnoreKind, envKind) {
			keys = append(keys, key)
		}
	}
	return keys
}

func envConfigForFeature(envConfig map[uuid.UUID]map[string][]featurepkg.MergedConfigRow, envID uuid.UUID, feature string) []featurepkg.MergedConfigRow {
	if m := envConfig[envID]; m != nil {
		return m[feature]
	}
	return nil
}

// mergeConfigRows overlays env rows on top of global rows, filtering by
// includeKeys. This mirrors featurepkg.MergeConfigs but works with
// pre-built MergedConfigRow slices instead of sqlc types.
func mergeConfigRows(global, env []featurepkg.MergedConfigRow, includeKeys []string) []featurepkg.MergedConfigRow {
	// Use the exported MergeConfigs by converting to sqlc types.
	// Since we already have MergedConfigRow, we do the merge inline.
	keySet := make(map[string]struct{}, len(includeKeys))
	for _, k := range includeKeys {
		keySet[k] = struct{}{}
	}

	m := make(map[string]featurepkg.MergedConfigRow, len(global)+len(env))
	for _, g := range global {
		if len(keySet) > 0 {
			if _, ok := keySet[g.Key]; !ok {
				continue
			}
		}
		m[g.Key] = g
	}
	for _, e := range env {
		if len(keySet) > 0 {
			if _, ok := keySet[e.Key]; !ok {
				continue
			}
		}
		m[e.Key] = e
	}

	result := make([]featurepkg.MergedConfigRow, 0, len(m))
	for _, v := range m {
		result = append(result, v)
	}
	slices.SortFunc(result, func(a, b featurepkg.MergedConfigRow) int {
		if a.Key < b.Key {
			return -1
		}
		if a.Key > b.Key {
			return 1
		}
		return 0
	})
	return result
}

func isErrMissingFields(err error, target **errs.ErrMissingRequiredFields) bool {
	return errors.As(err, target)
}

func generateHash(values map[string]any, feature *model.Feature) (string, error) {
	b, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	b = append(b, []byte(feature.Version+feature.Chart)...)
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:]), nil
}
