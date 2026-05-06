package features

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/deployment"
	envpkg "github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type DeploymentEnvStatus struct {
	Name               string
	TenantName         string
	TenantSlug         string
	Enabled            bool
	LastModified       time.Time
	LastDeployed       time.Time
	StatusText         string
	DeploymentID       string
	DeploymentVersion  string
	ReleaseVersion     string
	TargetLabels       map[string]string
	IsOverridden       bool
	OverriddenByID     string
	OverriddenByLabels map[string]string
}

type deploymentGroup struct {
	DeploymentID string
	Labels       map[string]string
	Environments []DeploymentEnvStatus
}

func loadDeploymentData(ctx context.Context, repo database.Repo, feature *model.Feature, data *DetailPage) {
	data.DeploymentEnvs = featureDeploymentEnvStatuses(ctx, repo, feature)
}

func deploymentStatusCounts(ctx context.Context, repo database.Repo, feature *model.Feature) (failed, pending int) {
	for _, env := range featureDeploymentEnvStatuses(ctx, repo, feature) {
		switch strings.ToUpper(env.StatusText) {
		case "FAILED":
			failed++
		case "PENDING", "CREATED":
			pending++
		}
	}
	return failed, pending
}

func deploymentDetailContent(data *DetailPage) g.Node {
	if len(data.DeploymentEnvs) == 0 {
		return h.P(g.Text("No environments found."))
	}
	groups := groupByDeployment(data.DeploymentEnvs)
	return deploymentStatusTable(groups, data.CurrentFeature.Name)
}

func groupByDeployment(envs []DeploymentEnvStatus) []deploymentGroup {
	groups := map[string]*deploymentGroup{}
	var order []string

	for _, env := range envs {
		if _, ok := groups[env.DeploymentID]; !ok {
			groups[env.DeploymentID] = &deploymentGroup{
				DeploymentID: env.DeploymentID,
				Labels:       env.TargetLabels,
			}
			order = append(order, env.DeploymentID)
		}
		groups[env.DeploymentID].Environments = append(groups[env.DeploymentID].Environments, env)
	}

	result := make([]deploymentGroup, 0, len(order))
	for _, id := range order {
		group := *groups[id]
		sort.SliceStable(group.Environments, func(i, j int) bool {
			a, b := group.Environments[i], group.Environments[j]
			if a.IsOverridden != b.IsOverridden {
				return !a.IsOverridden
			}
			if a.TenantName != b.TenantName {
				return a.TenantName < b.TenantName
			}
			return a.Name < b.Name
		})
		result = append(result, group)
	}
	return result
}

func deploymentStatusTable(groups []deploymentGroup, featureName string) g.Node {
	var bodies []g.Node
	for _, group := range groups {
		rows := []g.Node{
			h.Tr(h.Class("deployment-group-row"),
				h.Td(g.Attr("colspan", "7"),
					h.Div(h.Class("deployment-group-header"),
						h.A(h.Href("/deployments/"+group.DeploymentID), h.Class("deployment-group-link"),
							g.Text("Deployment "+group.DeploymentID[:8]),
						),
						labelPills(group.Labels),
					),
				),
			),
		}
		for _, env := range group.Environments {
			rows = append(rows, envRow(env, featureName))
		}
		bodies = append(bodies, h.TBody(g.Group(rows)))
	}
	return h.Table(h.Class("table"),
		h.THead(h.Tr(
			h.Th(g.Text("Tenant")),
			h.Th(g.Text("Environment")),
			h.Th(g.Text("Version")),
			h.Th(g.Text("Status")),
			h.Th(g.Text("Last update")),
			h.Th(g.Text("Last deployed")),
			h.Th(g.Text("")),
		)),
		g.Group(bodies),
	)
}

func envRow(env DeploymentEnvStatus, featureName string) g.Node {
	logsHref := "/tenants/" + env.TenantSlug + "/envs/" + env.Name + "/" + featureName + "/logs"
	envLink := h.A(h.Href("/tenants/"+env.TenantSlug+"/envs/"+env.Name+"/"+featureName), g.Text(env.Name))

	rowAttrs := []g.Node{}
	envCellChildren := []g.Node{envLink}
	if env.IsOverridden {
		rowAttrs = append(rowAttrs, h.Class("deployment-overridden"), h.Title(overrideTooltip(env)))
		envCellChildren = append(envCellChildren, h.Span(h.Class("text-muted override-note"), g.Text(" overridden")))
	}

	cells := []g.Node{
		h.Td(g.Text(env.TenantName)),
		h.Td(g.Group(envCellChildren)),
		h.Td(versionCell(env)),
		h.Td(rolloutStatus(env.StatusText)),
		h.Td(h.Title(view.FormatTime(env.LastModified)), g.Text(view.RelativeTime(env.LastModified))),
		lastDeployedCell(env.LastDeployed),
		h.Td(h.A(h.Href(logsHref), g.Attr("title", "View logs"), g.Text("📋"))),
	}
	return h.Tr(g.Group(append(rowAttrs, cells...)))
}

func overrideTooltip(env DeploymentEnvStatus) string {
	parts := []string{"Overridden by deployment " + shortID(env.OverriddenByID)}
	if len(env.OverriddenByLabels) > 0 {
		keys := make([]string, 0, len(env.OverriddenByLabels))
		for k := range env.OverriddenByLabels {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		pairs := make([]string, 0, len(keys))
		for _, k := range keys {
			pairs = append(pairs, k+"="+env.OverriddenByLabels[k])
		}
		parts = append(parts, "target: "+strings.Join(pairs, ", "))
	}
	return strings.Join(parts, " — ")
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func versionCell(env DeploymentEnvStatus) g.Node {
	if env.ReleaseVersion == "" {
		if env.DeploymentVersion == "" {
			return g.Text("")
		}
		return h.Span(h.Class("version-desired"), g.Text("→ "+env.DeploymentVersion))
	}
	if env.DeploymentVersion != "" && env.ReleaseVersion != env.DeploymentVersion {
		return h.Span(h.Class("version-mismatch"),
			g.Text(env.ReleaseVersion),
			h.Span(h.Class("version-desired"), g.Text("→ "+env.DeploymentVersion)),
		)
	}
	return g.Text(env.ReleaseVersion)
}

func labelPills(labels map[string]string) g.Node {
	if len(labels) == 0 {
		return h.Span(h.Class("label-filter-tag"), g.Text("All environments"))
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pills := make([]g.Node, 0, len(keys))
	for _, k := range keys {
		pills = append(pills, h.Span(h.Class("label-filter-tag"), g.Text(k+": "+labels[k])))
	}
	return g.Group(pills)
}

func featureDeploymentEnvStatuses(ctx context.Context, repo database.Repo, feature *model.Feature) []DeploymentEnvStatus {
	deployments, err := deployment.ListDeploymentsByFeature(ctx, feature.Name)
	if err != nil || len(deployments) == 0 {
		return []DeploymentEnvStatus{}
	}
	deployments = latestDeploymentPerTarget(deployments)

	tenants, err := envpkg.GetTenants(ctx)
	if err != nil {
		return []DeploymentEnvStatus{}
	}

	type envInfo struct {
		env        *model.Environment
		tenantName string
		labels     map[string]string
	}
	var allEnvs []envInfo
	for _, tenant := range tenants {
		envs, err := repo.EnvironmentsGet(ctx, tenant.ID)
		if err != nil {
			continue
		}
		for _, env := range envs {
			if !featureTargetsKind(feature.EnvironmentKinds, env.Kind) {
				continue
			}
			labels, err := repo.EnvironmentGetLabels(ctx, env.ID)
			if err != nil {
				continue
			}
			lblMap := make(map[string]string, len(labels))
			for _, l := range labels {
				lblMap[l.Key] = l.Value
			}
			allEnvs = append(allEnvs, envInfo{env: env, tenantName: tenant.Name, labels: lblMap})
		}
	}

	winners := map[uuid.UUID]*deployment.Deployment{}
	for _, env := range allEnvs {
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

	statusByDepEnv := map[string]*deployment.DeploymentStatus{}
	for _, dep := range deployments {
		statuses, err := deployment.ListDeploymentStatuses(ctx, dep.ID)
		if err != nil {
			continue
		}
		for _, status := range statuses {
			statusByDepEnv[dep.ID.String()+":"+status.EnvironmentID.String()] = status
		}
	}

	ret := []DeploymentEnvStatus{}
	for _, dep := range deployments {
		for _, env := range allEnvs {
			if !targetMatchesLabels(dep.TargetLabels, env.labels) {
				continue
			}

			state, err := featurepkg.FeatureStateGet(ctx, env.env.ID, feature.Name)
			if err != nil {
				continue
			}

			es := DeploymentEnvStatus{
				Name:              env.env.Name,
				TenantName:        env.tenantName,
				TenantSlug:        env.tenantName,
				Enabled:           state.Enabled,
				LastModified:      state.LastModified,
				DeploymentID:      dep.ID.String(),
				DeploymentVersion: dep.Feature.Version,
				TargetLabels:      dep.TargetLabels,
			}

			if di, err := repo.DeployInstructionsLatestDeployedForFeature(ctx, env.env.ID, feature.Name); err == nil && di != nil {
				es.LastDeployed = di.LastModified
			}

			releases, err := repo.ReleaseStatusesGet(ctx, env.env.ID)
			if err == nil {
				for _, release := range releases {
					if release.Name == feature.Name {
						es.ReleaseVersion = release.Version
						break
					}
				}
			}

			winner := winners[env.env.ID]
			if winner != nil && winner.ID != dep.ID {
				es.IsOverridden = true
				es.OverriddenByID = winner.ID.String()
				es.OverriddenByLabels = winner.TargetLabels
				es.StatusText = "OVERRIDDEN"
			} else if di, err := repo.DeployInstructionsLatestForFeature(ctx, env.env.ID, feature.Name); err == nil && di != nil {
				es.StatusText = strings.ToUpper(di.Status.String())
				es.LastModified = di.LastModified
			} else if status := statusByDepEnv[dep.ID.String()+":"+env.env.ID.String()]; status != nil {
				es.StatusText = string(status.State)
				es.LastModified = status.LastModified
			}

			ret = append(ret, es)
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
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+labels[k])
	}
	return strings.Join(parts, ",")
}

func targetMatchesLabels(target, envLabels map[string]string) bool {
	for k, v := range target {
		if envLabels[k] != v {
			return false
		}
	}
	return true
}
