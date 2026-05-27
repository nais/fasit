package features

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/deployment"
	envpkg "github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/ui/components"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type DeploymentEnvStatus struct {
	Name                 string
	TenantName           string
	TenantSlug           string
	Enabled              bool
	EnvReconcileDisabled bool
	LastModified         time.Time
	LastDeployed         time.Time
	StatusText           string
	DeploymentID         string
	DeploymentVersion    string
	ChartDescription     string
	ReleaseVersion       string
	TargetLabels         map[string]string
	IsOverridden         bool
	OverriddenByID       string
	OverriddenByVersion  string
	OverriddenByLabels   map[string]string
}

type ViewPrefs struct {
	Group          string // "tenant", "deployment", "version"
	ShowVersion    bool
	ShowLastDeploy bool
}

func overviewViewPrefs() ViewPrefs {
	return ViewPrefs{
		Group:          "tenant",
		ShowVersion:    true,
		ShowLastDeploy: true,
	}
}

func deploymentSpecsViewPrefs() ViewPrefs {
	return ViewPrefs{
		Group:          "deployment",
		ShowVersion:    false,
		ShowLastDeploy: true,
	}
}

type card struct {
	Title        string
	LinkHref     string
	Labels       map[string]string
	DeploymentID string
	Environments []DeploymentEnvStatus
}

func loadDeploymentData(ctx context.Context, feature *model.Feature, data *DetailPage) {
	data.DeploymentEnvs = featureDeploymentEnvStatuses(ctx, feature)
}

func deploymentDetailContent(data *DetailPage) g.Node {
	envs := currentDeploymentEnvStatuses(data.DeploymentEnvs)
	if len(envs) == 0 {
		return h.P(g.Text("No environments found."))
	}

	return overviewEnvironmentTable(envs, data.CurrentFeature.Name)
}

func overviewEnvironmentTable(envs []DeploymentEnvStatus, featureName string) g.Node {
	sortEnvs(envs)
	prefs := overviewViewPrefs()
	thNodes := []g.Node{
		h.Th(g.Text("Tenant")),
		h.Th(g.Text("Environment")),
		h.Th(g.Text("Status")),
		h.Th(g.Text("Version")),
		h.Th(h.Title("When the latest successful deployment instruction completed"), g.Text("Last successful deploy")),
		h.Th(h.Class("col-action"), g.Attr("data-no-sort", "")),
	}
	rows := g.Map(envs, func(env DeploymentEnvStatus) g.Node {
		return envCardRow(env, featureName, prefs, true, true, true)
	})
	table := h.Table(h.Class("table sortable"), g.Attr("data-sort-key", "feature-overview"),
		h.THead(h.Tr(g.Group(thNodes))),
		h.TBody(g.Group(rows)),
	)
	return h.Div(h.Class("feature-card feature-overview-table"),
		h.Div(h.Class("feature-card-body"), table),
	)
}

func currentDeploymentEnvStatuses(envs []DeploymentEnvStatus) []DeploymentEnvStatus {
	ret := make([]DeploymentEnvStatus, 0, len(envs))
	for _, env := range envs {
		if env.IsOverridden {
			continue
		}
		ret = append(ret, env)
	}
	return ret
}

func deploymentSpecsContent(data *DetailPage) g.Node {
	if len(data.DeploymentEnvs) == 0 {
		return h.P(g.Text("No environments found."))
	}

	prefs := deploymentSpecsViewPrefs()
	cards := buildCards(data.DeploymentEnvs, prefs.Group)
	return cardGrid(cards, data.CurrentFeature.Name, data.CurrentFeature.Chart, prefs)
}

func buildCards(envs []DeploymentEnvStatus, groupBy string) []card {
	switch groupBy {
	case "deployment":
		return groupByDeploymentCards(envs)
	case "version":
		return groupByVersionCards(envs)
	default:
		return groupByTenantCards(envs)
	}
}

func groupByTenantCards(envs []DeploymentEnvStatus) []card {
	groups := map[string]*card{}
	var order []string
	for _, env := range envs {
		if _, ok := groups[env.TenantName]; !ok {
			groups[env.TenantName] = &card{Title: env.TenantName}
			order = append(order, env.TenantName)
		}
		groups[env.TenantName].Environments = append(groups[env.TenantName].Environments, env)
	}
	sort.Strings(order)
	result := make([]card, 0, len(order))
	for _, name := range order {
		c := *groups[name]
		sortEnvs(c.Environments)
		result = append(result, c)
	}
	return result
}

func groupByDeploymentCards(envs []DeploymentEnvStatus) []card {
	groups := map[string]*card{}
	var order []string
	for _, env := range envs {
		if _, ok := groups[env.DeploymentID]; !ok {
			groups[env.DeploymentID] = &card{
				Title:        env.DeploymentVersion,
				LinkHref:     "/deployments/" + env.DeploymentID,
				Labels:       env.TargetLabels,
				DeploymentID: env.DeploymentID,
			}
			order = append(order, env.DeploymentID)
		}
		groups[env.DeploymentID].Environments = append(groups[env.DeploymentID].Environments, env)
	}
	result := make([]card, 0, len(order))
	for _, id := range order {
		c := *groups[id]
		sortEnvs(c.Environments)
		result = append(result, c)
	}
	return result
}

func groupByVersionCards(envs []DeploymentEnvStatus) []card {
	groups := map[string]*card{}
	var order []string
	for _, env := range envs {
		version := env.DeploymentVersion
		if env.IsOverridden && env.OverriddenByVersion != "" {
			version = env.OverriddenByVersion
		}
		if _, ok := groups[version]; !ok {
			groups[version] = &card{Title: version}
			order = append(order, version)
		}
		groups[version].Environments = append(groups[version].Environments, env)
	}
	sort.Strings(order)
	result := make([]card, 0, len(order))
	for _, v := range order {
		c := *groups[v]
		sortEnvs(c.Environments)
		result = append(result, c)
	}
	return result
}

func sortEnvs(envs []DeploymentEnvStatus) {
	sort.SliceStable(envs, func(i, j int) bool {
		a, b := envs[i], envs[j]
		if a.IsOverridden != b.IsOverridden {
			return !a.IsOverridden
		}
		if a.TenantName != b.TenantName {
			return a.TenantName < b.TenantName
		}
		return a.Name < b.Name
	})
}

func cardGrid(cards []card, featureName, chart string, prefs ViewPrefs) g.Node {
	if len(cards) == 0 {
		return h.P(h.Class("text-muted"), g.Text("No environments to show."))
	}
	return h.Div(h.Class("feature-card-grid"),
		g.Map(cards, func(c card) g.Node {
			return renderCard(c, featureName, chart, prefs)
		}),
	)
}

func renderCard(c card, featureName, chart string, prefs ViewPrefs) g.Node {
	heading := []g.Node{}

	if c.LinkHref != "" {
		heading = append(heading,
			h.A(h.Href(c.LinkHref), h.Class("card-group-title-link"), g.Text(c.Title)),
		)
	} else {
		heading = append(heading,
			h.Span(h.Class("card-group-title"), g.Text(c.Title)),
		)
	}

	if prefs.Group == "deployment" || len(c.Labels) > 0 {
		heading = append(heading, h.Span(h.Class("feature-card-labels"), labelPills(c.Labels)))
	}

	actions := []g.Node{}
	// Card-level kebab for deployment grouping (set version)
	if prefs.Group == "deployment" && c.DeploymentID != "" {
		popoverID := "set-version-" + c.DeploymentID
		actions = append(actions,
			h.Div(h.Class("card-kebab-wrap"),
				kebabButton("card-kebab-"+c.DeploymentID),
				h.Div(h.Class("kebab-menu"), h.ID("card-kebab-"+c.DeploymentID),
					h.Button(h.Type("button"), h.Class("kebab-item"),
						g.Attr("popovertarget", popoverID),
						g.Text("Set version"),
					),
				),
				setVersionPopover(popoverID, featureName, chart, c.Labels),
			),
		)
	}

	header := h.Div(h.Class("feature-card-header"),
		h.Div(h.Class("feature-card-heading"), g.Group(heading)),
		g.If(len(actions) > 0, h.Div(h.Class("feature-card-actions"), g.Group(actions))),
	)

	// Build table columns
	showTenant := prefs.Group != "tenant"
	showVersion := prefs.Group != "version" && prefs.ShowVersion

	thNodes := []g.Node{}
	if showTenant {
		thNodes = append(thNodes, h.Th(g.Text("Tenant")))
	}
	thNodes = append(thNodes, h.Th(g.Text("Environment")))
	thNodes = append(thNodes, h.Th(g.Text("Status")))
	if showVersion {
		thNodes = append(thNodes, h.Th(g.Text("Version")))
	}
	if prefs.ShowLastDeploy {
		thNodes = append(thNodes, h.Th(h.Title("When the latest successful deployment instruction completed"), g.Text("Last successful deploy")))
	}
	thNodes = append(thNodes, h.Th(h.Class("col-action"), g.Attr("data-no-sort", "")))

	rows := g.Map(c.Environments, func(env DeploymentEnvStatus) g.Node {
		return envCardRow(env, featureName, prefs, showTenant, showVersion, false)
	})

	table := h.Table(h.Class("table sortable"), g.Attr("data-sort-key", "feature-card"),
		h.THead(h.Tr(g.Group(thNodes))),
		h.TBody(g.Group(rows)),
	)

	return h.Div(h.Class("feature-card"),
		header,
		h.Div(h.Class("feature-card-body"), table),
	)
}

func envCardRow(env DeploymentEnvStatus, featureName string, prefs ViewPrefs, showTenant, showVersion, showTenantAvatar bool) g.Node {
	baseHref := "/features/" + featureName + "/envs/" + env.TenantSlug + "/" + env.Name
	envLink := h.A(h.Href(baseHref), g.Text(env.Name))
	logsHref := baseHref + "/logs"

	rowAttrs := []g.Node{}
	if env.IsOverridden {
		rowAttrs = append(rowAttrs, h.Class("deployment-overridden"))
	}

	cells := []g.Node{}
	if showTenant {
		cells = append(cells, tenantCell(env, showTenantAvatar))
	}

	hasDrift := env.ReleaseVersion != "" && env.ReleaseVersion != env.DeploymentVersion
	driftIcon := g.If(hasDrift,
		h.Span(
			h.Class("version-drift"),
			h.Title("Running: "+env.ReleaseVersion),
			g.Raw(` <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor"><path d="M8 1l7 14H1L8 1z" fill="#e8a735"/><text x="8" y="13" text-anchor="middle" font-size="10" font-weight="bold" fill="#000">!</text></svg>`),
		),
	)

	cells = append(cells, h.Td(envLink, g.If(!showVersion, driftIcon)))

	statusCell := []g.Node{}
	if tip := statusTooltip(env); tip != "" {
		statusCell = append(statusCell, h.Title(tip))
	}
	statusCell = append(statusCell, renderStatus(env.StatusText))
	cells = append(cells, h.Td(statusCell...))

	if showVersion {
		cells = append(cells, h.Td(g.Text(env.DeploymentVersion), driftIcon))
	}

	if prefs.ShowLastDeploy {
		cells = append(cells, lastDeployedCell(env.LastDeployed, ""))
	}

	// Kebab menu for row actions
	kebabID := "row-kebab-" + env.TenantSlug + "-" + env.Name
	cells = append(cells, h.Td(h.Class("action"),
		h.Div(h.Class("kebab-wrap"),
			kebabButton(kebabID),
			h.Div(h.Class("kebab-menu"), h.ID(kebabID),
				h.A(h.Href(logsHref), h.Class("kebab-item"), g.Text("View logs")),
			),
		),
	))

	return h.Tr(g.Group(append(rowAttrs, cells...)))
}

func tenantCell(env DeploymentEnvStatus, showAvatar bool) g.Node {
	if !showAvatar {
		return h.Td(g.Text(env.TenantName))
	}
	return h.Td(h.Span(h.Class("tenant-cell"),
		components.TenantAvatar(env.TenantName, components.HasTenantLogo(env.TenantName), "20px"),
		h.Span(g.Text(env.TenantName)),
	))
}

func kebabButton(targetID string) g.Node {
	return h.Button(
		h.Type("button"),
		h.Class("kebab-btn"),
		g.Attr("data-kebab-toggle", targetID),
		g.Attr("aria-label", "Actions"),
		g.Text("⋮"),
	)
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
	if env.EnvReconcileDisabled {
		tip := "Environment reconcile disabled"
		if env.ReleaseVersion != "" {
			tip += " — Running: " + env.ReleaseVersion
		}
		return tip
	}
	if !env.Enabled {
		tip := "Feature reconcile disabled"
		if env.ReleaseVersion != "" {
			tip += " — Running: " + env.ReleaseVersion
		}
		return tip
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

func featureDeploymentEnvStatuses(ctx context.Context, feature *model.Feature) []DeploymentEnvStatus {
	deployments, err := deployment.ListByFeature(ctx, feature.Name)
	if err != nil || len(deployments) == 0 {
		return []DeploymentEnvStatus{}
	}
	deployments = latestDeploymentPerTarget(deployments)

	tenants, err := envpkg.ListTenants(ctx)
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

			allEnvs = append(allEnvs, envInfo{env: env, tenantName: tenant.Name, labels: labels})
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

			disabledAt, disabled, err := featurepkg.FeatureDisabledAt(ctx, env.env.ID, feature.Name)
			if err != nil {
				continue
			}

			es := DeploymentEnvStatus{
				Name:                 env.env.Name,
				TenantName:           env.tenantName,
				TenantSlug:           env.tenantName,
				Enabled:              !disabled,
				EnvReconcileDisabled: !env.env.Reconcile,
				DeploymentID:         dep.ID.String(),
				DeploymentVersion:    dep.Feature.Version,
				ChartDescription:     dep.Feature.Description,
				TargetLabels:         dep.TargetLabels,
			}
			if disabled {
				es.LastModified = disabledAt
			}

			if di, err := featurepkg.GetLatestDeployedDeployInstruction(ctx, env.env.ID, feature.Name); err == nil && di != nil {
				es.LastDeployed = di.LastModified
			}

			releases, err := deployment.ListReleaseStatuses(ctx, env.env.ID)
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
			} else if !env.env.Reconcile {
				es.StatusText = "DISABLED"
			} else if disabled {
				es.StatusText = "DISABLED"
				es.LastModified = disabledAt
			} else {
				es.StatusText = "UNKNOWN"
				if status := statusByDepEnv[dep.ID.String()+":"+env.env.ID.String()]; status != nil {
					es.StatusText = deployment.NormalizeStatus(string(status.State))
					es.LastModified = status.LastModified
				}
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
