package featureworkspace

import (
	"context"
	"slices"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/deployment"
	envpkg "github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/ui/uidata"
)

type Environment struct {
	TenantName           string
	TenantSlug           string
	EnvironmentName      string
	Enabled              bool
	DisableReason        string
	EnvReconcileDisabled bool
	LastModified         time.Time
	Status               string
}

func LoadEnvironments(ctx context.Context, feature *model.Feature) []Environment {
	deployments, err := deployment.ListByFeature(ctx, feature.Name)
	if err != nil || len(deployments) == 0 {
		return []Environment{}
	}
	deployments = latestDeploymentPerTarget(deployments)

	targetedEnvs := targetedEnvironments(ctx, feature)
	winners := winningDeployments(deployments, targetedEnvs)
	statusByDepEnv := deploymentStatuses(ctx, deployments)

	ret := make([]Environment, 0, len(winners))
	for _, env := range targetedEnvs {
		winner := winners[env.env.ID]
		if winner == nil {
			continue
		}

		disabledAt, disabled, err := featurepkg.FeatureDisabledAt(ctx, env.env.ID, feature.Name)
		if err != nil {
			continue
		}

		status := Environment{
			TenantName:           env.tenantName,
			TenantSlug:           env.tenantName,
			EnvironmentName:      env.env.Name,
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
		case statusByDepEnv[winner.ID.String()+":"+env.env.ID.String()] != nil:
			deploymentStatus := statusByDepEnv[winner.ID.String()+":"+env.env.ID.String()]
			status.Status = deployment.NormalizeStatus(string(deploymentStatus.State))
			status.LastModified = deploymentStatus.LastModified
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
	env        *model.Environment
	tenantName string
	labels     map[string]string
}

func targetedEnvironments(ctx context.Context, feature *model.Feature) []envInfo {
	tenants, err := uidata.ListTenants(ctx)
	if err != nil {
		return nil
	}

	var ret []envInfo
	for _, tenant := range tenants {
		envs, err := envpkg.List(ctx, tenant.ID)
		if err != nil {
			continue
		}
		for _, env := range envs {
			if !featureTargetsKind(feature.EnvironmentKinds, env.Kind) {
				continue
			}
			labels, err := envpkg.GetLabels(ctx, env.ID)
			if err != nil {
				continue
			}
			ret = append(ret, envInfo{env: env, tenantName: tenant.Name, labels: labels})
		}
	}
	return ret
}

func winningDeployments(deployments []*deployment.Deployment, envs []envInfo) map[uuid.UUID]*deployment.Deployment {
	winners := map[uuid.UUID]*deployment.Deployment{}
	for _, env := range envs {
		for _, dep := range deployments {
			if !targetMatchesLabels(dep.TargetLabels, env.labels) {
				continue
			}
			existing, ok := winners[env.env.ID]
			if !ok || deployment.IsMoreSpecific(dep.TargetLabels, existing.TargetLabels, dep.Created, existing.Created) {
				winners[env.env.ID] = dep
			}
		}
	}
	return winners
}

func deploymentStatuses(ctx context.Context, deployments []*deployment.Deployment) map[string]*deployment.DeploymentStatus {
	ret := map[string]*deployment.DeploymentStatus{}
	for _, dep := range deployments {
		statuses, err := deployment.ListDeploymentStatuses(ctx, dep.ID)
		if err != nil {
			continue
		}
		for _, status := range statuses {
			ret[dep.ID.String()+":"+status.EnvironmentID.String()] = status
		}
	}
	return ret
}

func latestDeploymentPerTarget(deps []*deployment.Deployment) []*deployment.Deployment {
	seen := map[string]struct{}{}
	ret := make([]*deployment.Deployment, 0, len(deps))
	for _, dep := range deps {
		key := targetKey(dep.TargetLabels)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		ret = append(ret, dep)
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

func featureTargetsKind(kinds []model.EnvironmentKind, envKind model.EnvironmentKind) bool {
	if len(kinds) == 0 {
		return true
	}
	return slices.Contains(kinds, envKind)
}
