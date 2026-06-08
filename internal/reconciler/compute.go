package reconciler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	environment2 "github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/errs"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/graph/model"
)

func (r *Reconciler) computeActions(snap *snapshot) []DeployDecision {
	out := make(chan DeployDecision, 2048)
	go r.computeActionsStream(snap, out)
	var results []DeployDecision
	for d := range out {
		results = append(results, d)
	}
	return results
}

// workItem is sent through the work channel to a worker goroutine.
type workItem struct {
	env environment
	dep *reconcileAssignment
}

// computeActionsStream dispatches deploy decisions to out and closes it when
// done. Skips (unhealthy/disabled) go directly to out; compute work is
// processed by a pool of GOMAXPROCS workers. Caller must consume out.
func (r *Reconciler) computeActionsStream(snap *snapshot, out chan<- DeployDecision) {
	numWorkers := runtime.GOMAXPROCS(0)
	work := make(chan workItem, numWorkers)

	// Workers: read work items, compute, send to out.
	var wg sync.WaitGroup
	for range numWorkers {
		wg.Go(func() {
			for w := range work {
				out <- r.computeAction(snap, w.env, w.dep)
			}
		})
	}

	// Dispatch: send skips directly to out, compute work to workers.
	for _, env := range snap.environments {
		reportedAt, ok := snap.healthByEnv[env.ID]
		healthy := ok && time.Since(reportedAt) <= 3*time.Minute

		matched := matchAssignments(snap.assignments, env)
		winners := mostSpecificPerFeature(matched)

		for _, dep := range winners {
			if !healthy {
				out <- DeployDecision{
					EnvironmentID:       env.ID,
					EnvironmentName:     env.Name,
					TenantName:          env.TenantName,
					FeatureAssignmentID: dep.ID,
					Feature:             dep.Feature,
					Action:              ActionSkipUnhealthy,
					Message:             "naisd is unhealthy",
				}
				continue
			}

			if snap.disabledByEnv[env.ID][dep.Feature.Name] {
				out <- DeployDecision{
					EnvironmentID:       env.ID,
					EnvironmentName:     env.Name,
					TenantName:          env.TenantName,
					FeatureAssignmentID: dep.ID,
					Feature:             dep.Feature,
					Action:              ActionSkipDisabled,
					Message:             "feature reconcile disabled",
				}
				continue
			}

			work <- workItem{env: env, dep: dep}
		}
	}

	close(work) // workers drain remaining items then exit
	wg.Wait()   // all results sent to out
	close(out)  // signal consumer we're done
}

func (r *Reconciler) computeAction(snap *snapshot, env environment, dep *reconcileAssignment) DeployDecision {
	base := DeployDecision{
		EnvironmentID:       env.ID,
		EnvironmentName:     env.Name,
		TenantName:          env.TenantName,
		FeatureAssignmentID: dep.ID,
		Feature:             dep.Feature,
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
	// Clone mv because GenerateWith sets mv.Configs and sprig template
	// functions like set/unset can mutate Env/Management/Envs maps.
	mvCopy := *mv
	mvCopy.Env = copyStringAnyMap(mv.Env)
	mvCopy.Management = copyStringAnyMap(mv.Management)
	mvCopy.Envs = make([]map[string]any, len(mv.Envs))
	for i, e := range mv.Envs {
		mvCopy.Envs[i] = copyStringAnyMap(e)
	}

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
		if instr.Status == model.RolloutStatusPending.String() {
			base.Action = ActionSkipInProgress
			base.Status = instr.Status
			base.Message = "deployment is already in progress"
			return base
		}
		if instr.Hash == hash {
			base.Action = ActionSkipUnchanged
			base.Status = instr.Status
			base.Message = "no changes in feature"
			return base
		}
	}

	base.Action = ActionDeploy
	base.Message = "deployment instruction sent to naisd"
	return base
}

func matchAssignments(all []*reconcileAssignment, env environment) []*reconcileAssignment {
	var matched []*reconcileAssignment
	for _, dep := range all {
		if labelsContain(env.Labels, dep.TargetLabels) {
			matched = append(matched, dep)
		}
	}
	return matched
}

func mostSpecificPerFeature(deps []*reconcileAssignment) []*reconcileAssignment {
	winners := map[string]*reconcileAssignment{}
	for _, dep := range deps {
		existing, ok := winners[dep.Feature.Name]
		if !ok || featureassignment.IsMoreSpecific(dep.TargetLabels, existing.TargetLabels, dep.Created, existing.Created) {
			winners[dep.Feature.Name] = dep
		}
	}

	result := make([]*reconcileAssignment, 0, len(winners))
	for _, d := range winners {
		result = append(result, d)
	}
	slices.SortStableFunc(result, func(a, b *reconcileAssignment) int {
		return a.Created.Compare(b.Created)
	})
	return result
}

func checkDependencies(f *featurepkg.Feature, envID uuid.UUID, deployedFeats map[uuid.UUID]map[string]bool) (bool, []string) {
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

func configIncludeKeys(f *featurepkg.Feature, envKind environment2.EnvironmentKind) []string {
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

func copyStringAnyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	c := make(map[string]any, len(m))
	maps.Copy(c, m)
	return c
}

func isErrMissingFields(err error, target **errs.ErrMissingRequiredFields) bool {
	return errors.As(err, target)
}

func generateHash(values map[string]any, feature *featurepkg.Feature) (string, error) {
	b, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	b = append(b, []byte(feature.Version+feature.Chart)...)
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:]), nil
}
