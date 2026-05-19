package features

import (
	"context"
	"net/http"
	"net/url"
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
	ChartDescription    string
	ReleaseVersion      string
	TargetLabels        map[string]string
	IsOverridden        bool
	OverriddenByID      string
	OverriddenByVersion string
	OverriddenByLabels  map[string]string
}

type ViewPrefs struct {
	Group              string // "tenant", "deployment", "desired_version", "current_version"
	ShowOverridden     bool
	ShowDesiredVersion bool
	ShowCurrentVersion bool
	ShowLastDeploy     bool
	ShowLastUpdate     bool
}

var validGroups = map[string]bool{
	"tenant": true, "deployment": true, "desired_version": true, "current_version": true,
}

func parseViewPrefs(r *http.Request) ViewPrefs {
	p := ViewPrefs{
		Group:              "tenant",
		ShowOverridden:     false,
		ShowDesiredVersion: true,
		ShowCurrentVersion: true,
		ShowLastDeploy:     true,
		ShowLastUpdate:     true,
	}

	// Use query params if present, otherwise fall back to cookie.
	src := r.URL.Query()
	if len(src) == 0 {
		if c, err := r.Cookie("feature_view_prefs"); err == nil {
			if decoded, err := url.QueryUnescape(c.Value); err == nil {
				if parsed, err := url.ParseQuery(decoded); err == nil {
					src = parsed
				}
			}
		}
	}

	if g := src.Get("group"); validGroups[g] {
		p.Group = g
	}
	if src.Get("show_overridden") == "true" {
		p.ShowOverridden = true
	}
	if src.Has("col_desired_version") {
		p.ShowDesiredVersion = src.Get("col_desired_version") == "true"
	}
	if src.Has("col_current_version") {
		p.ShowCurrentVersion = src.Get("col_current_version") == "true"
	}
	if src.Has("col_last_deployed") {
		p.ShowLastDeploy = src.Get("col_last_deployed") == "true"
	}
	if src.Has("col_last_updated") {
		p.ShowLastUpdate = src.Get("col_last_updated") == "true"
	}
	return p
}

type card struct {
	Title        string
	LinkHref     string
	Labels       map[string]string
	DeploymentID string
	Environments []DeploymentEnvStatus
}

func loadDeploymentData(ctx context.Context, repo database.Repo, feature *model.Feature, data *DetailPage) {
	data.DeploymentEnvs = featureDeploymentEnvStatuses(ctx, repo, feature)
	for _, env := range data.DeploymentEnvs {
		if env.IsOverridden {
			continue
		}
		data.ChartDescriptions = append(data.ChartDescriptions, env.ChartDescription)
	}
}

func deploymentDetailContent(data *DetailPage) g.Node {
	if len(data.DeploymentEnvs) == 0 {
		return h.P(g.Text("No environments found."))
	}

	prefs := data.Prefs
	envs := data.DeploymentEnvs

	// Filter overridden rows: only shown in deployment grouping when toggled on
	if prefs.Group != "deployment" || !prefs.ShowOverridden {
		filtered := make([]DeploymentEnvStatus, 0, len(envs))
		for _, env := range envs {
			if !env.IsOverridden {
				filtered = append(filtered, env)
			}
		}
		envs = filtered
	}

	cards := buildCards(envs, prefs.Group)

	return h.Div(
		toolbar(prefs),
		cardGrid(cards, data.CurrentFeature.Name, data.CurrentFeature.Chart, prefs),
	)
}

func toolbar(prefs ViewPrefs) g.Node {
	groupOptions := []struct{ Value, Label string }{
		{"tenant", "Tenant"},
		{"deployment", "Deployment"},
		{"desired_version", "Desired version"},
		{"current_version", "Current version"},
	}

	groupLinks := h.Div(h.Class("toolbar-group-links"),
		g.Map(groupOptions, func(opt struct{ Value, Label string }) g.Node {
			cls := "toolbar-group-link"
			if opt.Value == prefs.Group {
				cls += " active"
			}
			return h.Button(
				h.Type("button"),
				h.Class(cls),
				g.Attr("data-group-value", opt.Value),
				g.Text(opt.Label),
			)
		}),
	)

	overriddenToggle := g.If(prefs.Group == "deployment",
		h.Label(h.Class("toolbar-toggle"),
			h.Input(
				h.Type("checkbox"),
				g.Attr("data-pref", "show_overridden"),
				g.If(prefs.ShowOverridden, g.Attr("checked", "")),
			),
			g.Text(" Show overridden"),
		),
	)

	colMenu := h.Div(h.Class("col-toggle-wrap"),
		h.Button(h.Type("button"), h.Class("col-toggle-btn"),
			g.Attr("data-col-toggle", "col-toggle-menu"),
			g.Attr("aria-label", "Toggle columns"),
			g.Attr("title", "Columns"),
			// Three-column icon (SVG)
			g.Raw(`<svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><rect x="1" y="2" width="3" height="12" rx="0.5"/><rect x="6.5" y="2" width="3" height="12" rx="0.5"/><rect x="12" y="2" width="3" height="12" rx="0.5"/></svg>`),
		),
		h.Div(h.Class("col-toggle-menu"), h.ID("col-toggle-menu"),
			colToggleItem("Desired version", "col_desired_version", prefs.ShowDesiredVersion),
			colToggleItem("Current version", "col_current_version", prefs.ShowCurrentVersion),
			colToggleItem("Last updated", "col_last_updated", prefs.ShowLastUpdate),
			colToggleItem("Last deployed", "col_last_deployed", prefs.ShowLastDeploy),
		),
	)

	return h.Div(h.Class("view-toolbar"), h.ID("view-toolbar"),
		h.Span(h.Class("toolbar-label"), g.Text("Group by")),
		groupLinks,
		overriddenToggle,
		colMenu,
	)
}

func colToggleItem(label, prefKey string, checked bool) g.Node {
	return h.Label(h.Class("col-toggle-item"),
		h.Input(
			h.Type("checkbox"),
			g.Attr("data-pref", prefKey),
			g.If(checked, g.Attr("checked", "")),
		),
		g.Text(" "+label),
	)
}

func buildCards(envs []DeploymentEnvStatus, groupBy string) []card {
	switch groupBy {
	case "deployment":
		return groupByDeploymentCards(envs)
	case "desired_version":
		return groupByDesiredVersionCards(envs)
	case "current_version":
		return groupByCurrentVersionCards(envs)
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

func groupByDesiredVersionCards(envs []DeploymentEnvStatus) []card {
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

func groupByCurrentVersionCards(envs []DeploymentEnvStatus) []card {
	groups := map[string]*card{}
	var order []string
	for _, env := range envs {
		version := env.ReleaseVersion
		if version == "" {
			version = "(none)"
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
	headerChildren := []g.Node{}

	if c.LinkHref != "" {
		headerChildren = append(headerChildren,
			h.A(h.Href(c.LinkHref), h.Class("card-group-title-link"), g.Text(c.Title)),
		)
	} else {
		headerChildren = append(headerChildren,
			h.Span(h.Class("card-group-title"), g.Text(c.Title)),
		)
	}

	if len(c.Labels) > 0 {
		headerChildren = append(headerChildren, labelPills(c.Labels))
	}

	// Card-level kebab for deployment grouping (set version)
	if prefs.Group == "deployment" && c.DeploymentID != "" {
		popoverID := "set-version-" + c.DeploymentID
		headerChildren = append(headerChildren,
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

	header := h.Div(h.Class("feature-card-header"), g.Group(headerChildren))

	// Build table columns
	showTenant := prefs.Group != "tenant"
	showDesired := prefs.Group != "desired_version" && prefs.ShowDesiredVersion
	showCurrent := prefs.Group != "current_version" && prefs.ShowCurrentVersion

	thNodes := []g.Node{}
	if showTenant {
		thNodes = append(thNodes, h.Th(g.Text("Tenant")))
	}
	thNodes = append(thNodes, h.Th(g.Text("Environment")))
	thNodes = append(thNodes, h.Th(g.Text("Status")))
	if showDesired {
		thNodes = append(thNodes, h.Th(g.Text("Desired")))
	}
	if showCurrent {
		thNodes = append(thNodes, h.Th(g.Text("Current")))
	}
	if prefs.ShowLastUpdate {
		thNodes = append(thNodes, h.Th(g.Text("Last updated")))
	}
	if prefs.ShowLastDeploy {
		thNodes = append(thNodes, h.Th(g.Text("Last deployed")))
	}
	thNodes = append(thNodes, h.Th(h.Class("col-action"), g.Attr("data-no-sort", "")))

	rows := g.Map(c.Environments, func(env DeploymentEnvStatus) g.Node {
		return envCardRow(env, featureName, prefs, showTenant, showDesired, showCurrent)
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

func envCardRow(env DeploymentEnvStatus, featureName string, prefs ViewPrefs, showTenant, showDesired, showCurrent bool) g.Node {
	envLink := h.A(h.Href("/tenants/"+env.TenantSlug+"/envs/"+env.Name+"/"+featureName), g.Text(env.Name))
	logsHref := "/tenants/" + env.TenantSlug + "/envs/" + env.Name + "/" + featureName + "/logs"

	rowAttrs := []g.Node{}
	if env.IsOverridden {
		rowAttrs = append(rowAttrs, h.Class("deployment-overridden"))
	}

	cells := []g.Node{}
	if showTenant {
		cells = append(cells, h.Td(g.Text(env.TenantName)))
	}
	cells = append(cells, h.Td(envLink))

	statusCell := []g.Node{}
	if tip := statusTooltip(env); tip != "" {
		statusCell = append(statusCell, h.Title(tip))
	}
	statusCell = append(statusCell, renderStatus(env.StatusText))
	cells = append(cells, h.Td(statusCell...))

	if showDesired {
		cells = append(cells, h.Td(g.Text(env.DeploymentVersion)))
	}
	if showCurrent {
		version := env.ReleaseVersion
		if version == "" {
			cells = append(cells, h.Td(h.Span(h.Class("text-muted"), g.Text("—"))))
		} else {
			cells = append(cells, h.Td(g.Text(version)))
		}
	}

	if prefs.ShowLastUpdate {
		cells = append(cells, h.Td(h.Title(view.FormatTime(env.LastModified)), g.Text(view.RelativeTime(env.LastModified))))
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

			disabledAt, disabled, err := featurepkg.FeatureDisabledAt(ctx, env.env.ID, feature.Name)
			if err != nil {
				continue
			}

			es := DeploymentEnvStatus{
				Name:              env.env.Name,
				TenantName:        env.tenantName,
				TenantSlug:        env.tenantName,
				Enabled:           !disabled,
				DeploymentID:      dep.ID.String(),
				DeploymentVersion: dep.Feature.Version,
				ChartDescription:  dep.Feature.Description,
				TargetLabels:      dep.TargetLabels,
			}
			if disabled {
				es.LastModified = disabledAt
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
