package featureenvs

import (
	"context"
	"slices"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/audit"
	envpkg "github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/reconciler"
)

type Environment struct {
	TenantName           string
	TenantSlug           string
	EnvironmentName      string
	Version              string
	AssignmentID         string
	Enabled              bool
	DisableReason        string
	EnvReconcileDisabled bool
	LastModified         time.Time
	Status               string
}

func LoadEnvironments(ctx context.Context, feature *featurepkg.Feature) []Environment {
	assignments, err := featureassignment.ListByFeature(ctx, feature.Name)
	if err != nil || len(assignments) == 0 {
		return []Environment{}
	}

	if len(assignments) == 0 {
		return []Environment{}
	}
	assignments = latestAssignmentPerTarget(assignments)

	targetedEnvs := targetedEnvironments(ctx, feature)
	winners := winningAssignments(assignments, targetedEnvs)
	statusByAssignmentEnv := reconcileStatuses(ctx, assignments)

	disabledByEnv, err := featurepkg.DisabledEnvironmentsForFeature(ctx, feature.Name)
	if err != nil {
		return []Environment{}
	}

	ret := make([]Environment, 0, len(winners))
	for _, env := range targetedEnvs {
		winner := winners[env.env.ID]
		if winner == nil {
			continue
		}

		disabledAt, disabled := disabledByEnv[env.env.ID]

		status := Environment{
			TenantName:           env.tenantName,
			TenantSlug:           env.tenantName,
			EnvironmentName:      env.env.Name,
			Version:              winner.Feature.Version,
			AssignmentID:         winner.ID.String(),
			Enabled:              !disabled,
			EnvReconcileDisabled: !env.env.Reconcile,
			Status:               "UNKNOWN",
		}
		if disabled {
			status.LastModified = disabledAt
			status.DisableReason = audit.LatestDisableReason(ctx, feature.Name, env.env.ID)
		}

		switch {
		case !env.env.Reconcile:
			status.Status = "DISABLED"
		case disabled:
			status.Status = "DISABLED"
		case statusByAssignmentEnv[winner.ID.String()+":"+env.env.ID.String()] != nil:
			reconcileStatus := statusByAssignmentEnv[winner.ID.String()+":"+env.env.ID.String()]
			status.Status = reconciler.NormalizeStatus(string(reconcileStatus.State))
			status.LastModified = reconcileStatus.LastModified
		}

		ret = append(ret, status)
	}

	sort.Slice(ret, func(i, j int) bool {
		if ret[i].TenantName == ret[j].TenantName {
			return ret[i].EnvironmentName < ret[j].EnvironmentName
		}
		return ret[i].TenantName < ret[j].TenantName
	})
	return ret
}

type envInfo struct {
	env        *envpkg.Environment
	tenantName string
	labels     map[string]string
}

func targetedEnvironments(ctx context.Context, feature *featurepkg.Feature) []envInfo {
	envs, err := envpkg.ListTenantEnvironments(ctx, false)
	if err != nil {
		return nil
	}

	var ret []envInfo
	for _, te := range envs {
		if !featureTargetsKind(feature.EnvironmentKinds, te.Kind) {
			continue
		}
		env := te.Environment
		ret = append(ret, envInfo{env: &env, tenantName: te.TenantName, labels: te.Labels})
	}
	return ret
}

func winningAssignments(assignments []*featureassignment.FeatureAssignment, envs []envInfo) map[uuid.UUID]*featureassignment.FeatureAssignment {
	winners := map[uuid.UUID]*featureassignment.FeatureAssignment{}
	for _, env := range envs {
		for _, fa := range assignments {
			if !targetMatchesLabels(fa.TargetLabels, env.labels) {
				continue
			}
			existing, ok := winners[env.env.ID]
			if !ok || featureassignment.IsMoreSpecific(fa.TargetLabels, existing.TargetLabels, fa.Created, existing.Created) {
				winners[env.env.ID] = fa
			}
		}
	}
	return winners
}

func reconcileStatuses(ctx context.Context, assignments []*featureassignment.FeatureAssignment) map[string]*reconciler.FeatureReconcileStatus {
	ret := map[string]*reconciler.FeatureReconcileStatus{}
	statusesByFA, err := reconciler.AllReconcileStatuses(ctx)
	if err != nil {
		return ret
	}
	for _, fa := range assignments {
		for _, status := range statusesByFA[fa.ID] {
			ret[fa.ID.String()+":"+status.EnvironmentID.String()] = status
		}
	}
	return ret
}

func latestAssignmentPerTarget(fas []*featureassignment.FeatureAssignment) []*featureassignment.FeatureAssignment {
	seen := map[string]struct{}{}
	ret := make([]*featureassignment.FeatureAssignment, 0, len(fas))
	for _, fa := range fas {
		key := targetKey(fa.TargetLabels)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		ret = append(ret, fa)
	}
	return ret
}

func targetKey(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	ret := ""
	for _, k := range keys {
		ret += k + "=" + labels[k] + ";"
	}
	return ret
}

func targetMatchesLabels(target, envLabels map[string]string) bool {
	for k, v := range target {
		if envLabels[k] != v {
			return false
		}
	}
	return true
}

func featureTargetsKind(kinds []envpkg.EnvironmentKind, envKind envpkg.EnvironmentKind) bool {
	if len(kinds) == 0 {
		return true
	}
	return slices.Contains(kinds, envKind)
}
