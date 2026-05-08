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
	Name                string
	TenantName          string
	TenantSlug          string
	Enabled             bool
	LastModified        time.Time
	LastDeployed        time.Time
	StatusText          string
	DeploymentID        string
	DeploymentVersion   string
	ReleaseVersion      string
	TargetLabels        map[string]string
	IsOverridden        bool
	OverriddenByID      string
	OverriddenByVersion string
	OverriddenByLabels  map[string]string
}

type deploymentGroup struct {
	DeploymentID string
	Version      string
	Labels       map[string]string
	Environments []DeploymentEnvStatus
}

func loadDeploymentData(ctx context.Context, repo database.Repo, feature *model.Feature, data *DetailPage) {
	data.DeploymentEnvs = featureDeploymentEnvStatuses(ctx, repo, feature)
}

func deploymentDetailContent(data *DetailPage) g.Node {
	if len(data.DeploymentEnvs) == 0 {
		return h.P(g.Text("No environments found."))
	}
	groups := groupByDeployment(data.DeploymentEnvs)
	return deploymentStatusTable(groups, data.CurrentFeature.Name, data.CurrentFeature.Chart)
}

func groupByDeployment(envs []DeploymentEnvStatus) []deploymentGroup {
	groups := map[string]*deploymentGroup{}
	var order []string

	for _, env := range envs {
		if _, ok := groups[env.DeploymentID]; !ok {
			groups[env.DeploymentID] = &deploymentGroup{
				DeploymentID: env.DeploymentID,
				Version:      env.DeploymentVersion,
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

func deploymentStatusTable(groups []deploymentGroup, featureName, chart string) g.Node {
	var bodies []g.Node
	for _, group := range groups {
		popoverID := "set-version-" + group.DeploymentID
		headerChildren := []g.Node{}
		if group.Version != "" {
			headerChildren = append(headerChildren,
				h.A(h.Href("/deployments/"+group.DeploymentID), h.Class("deployment-group-version"), h.Title("View deployment"), g.Text(group.Version)),
			)
		}
		headerChildren = append(headerChildren, labelPills(group.Labels))
		headerChildren = append(headerChildren,
			h.Button(h.Type("button"), h.Class("btn-small set-version-btn"), g.Attr("popovertarget", popoverID), g.Text("Set version")),
			setVersionPopover(popoverID, featureName, chart, group.Labels),
		)
		rows := []g.Node{
			h.Tr(h.Class("deployment-group-row"), h.ID("deployment-"+group.DeploymentID),
				h.Td(g.Attr("colspan", "6"),
					h.Div(h.Class("deployment-group-header"), g.Group(headerChildren)),
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
	rowTip := ""
	if env.IsOverridden {
		rowAttrs = append(rowAttrs, h.Class("deployment-overridden"), g.Attr("data-overridden-by", env.OverriddenByID))
		rowTip = statusTooltip(env)
	}

	td := func(children ...g.Node) g.Node {
		if rowTip != "" {
			children = append([]g.Node{h.Title(rowTip)}, children...)
		}
		return h.Td(children...)
	}

	statusCell := []g.Node{}
	if tip := statusTooltip(env); tip != "" && rowTip == "" {
		statusCell = append(statusCell, h.Title(tip))
	}
	statusCell = append(statusCell, rolloutStatus(env.StatusText))

	cells := []g.Node{
		td(g.Text(env.TenantName)),
		td(envLink),
		td(statusCell...),
		td(h.Title(view.FormatTime(env.LastModified)), g.Text(view.RelativeTime(env.LastModified))),
		lastDeployedCell(env.LastDeployed, rowTip),
		td(h.A(h.Href(logsHref), g.Attr("title", "View logs"), g.Text("📋"))),
	}
	return h.Tr(g.Group(append(rowAttrs, cells...)))
}

func statusTooltip(env DeploymentEnvStatus) string {
	if env.IsOverridden {
		parts := []string{}
		if env.OverriddenByVersion != "" {
			parts = append(parts, "Overridden by "+env.OverriddenByVersion)
		} else {
			parts = append(parts, "Overridden")
		}
		if labels := formatLabels(env.OverriddenByLabels); labels != "" {
			parts = append(parts, "target: "+labels)
		}
		return strings.Join(parts, " — ")
	}
	if env.ReleaseVersion != "" && env.ReleaseVersion != env.DeploymentVersion {
		return "Currently: " + env.ReleaseVersion
	}
	if strings.HasPrefix(env.StatusText, "PENDING") && env.ReleaseVersion == "" {
		return "No release deployed yet"
	}
	return ""
}

func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, k+"="+labels[k])
	}
	return strings.Join(pairs, ", ")
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func setVersionPopover(popoverID, featureName, chart string, target map[string]string) g.Node {
	keys := make([]string, 0, len(target))
	for k := range target {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	inputs := []g.Node{
		h.Input(h.Type("hidden"), h.Name("feature_name"), h.Value(featureName)),
		h.Input(h.Type("hidden"), h.Name("chart"), h.Value(chart)),
	}
	for _, k := range keys {
		inputs = append(inputs, h.Input(h.Type("hidden"), h.Name("target_label"), h.Value(k+"="+target[k])))
	}
	return h.Div(g.Attr("popover", ""), h.ID(popoverID),
		h.H3(g.Text("Set version")),
		h.Form(h.Method("POST"), h.Action("/deployments"),
			g.Group(inputs),
			h.Label(g.Text("Version")),
			h.Input(h.Type("text"), h.Name("version"), g.Attr("required", ""), g.Attr("autofocus", "")),
			h.Div(h.Class("popover-actions"),
				h.Button(h.Type("submit"), g.Text("Set version")),
				h.Button(h.Type("button"), g.Attr("popovertarget", popoverID), g.Attr("popovertargetaction", "hide"), g.Text("Cancel")),
			),
		),
	)
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
				es.OverriddenByVersion = winner.Feature.Version
				es.OverriddenByLabels = winner.TargetLabels
				es.StatusText = "OVERRIDDEN"
			} else {
				fallbackState := ""
				var fallbackModified time.Time
				if status := statusByDepEnv[dep.ID.String()+":"+env.env.ID.String()]; status != nil {
					fallbackState = string(status.State)
					fallbackModified = status.LastModified
				}
				es.StatusText, es.LastModified = view.EffectiveDeploymentStatus(ctx, repo, env.env.ID, feature.Name, fallbackState, fallbackModified, es.DeploymentVersion, es.ReleaseVersion)
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
