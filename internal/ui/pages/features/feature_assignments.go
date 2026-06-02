package features

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/audit"
	envpkg "github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/pages/environment"
	"github.com/nais/fasit/internal/ui/uidata"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type AssignmentEnvStatus struct {
	Name                 string
	TenantName           string
	TenantSlug           string
	Enabled              bool
	DisableReason        string
	EnvReconcileDisabled bool
	LastModified         time.Time
	LastDeployed         time.Time
	StatusText           string
	FeatureAssignmentID  string
	AssignmentVersion    string
	ChartDescription     string
	ReleaseVersion       string
	TargetLabels         map[string]string
	IsOverridden         bool
	OverriddenByID       string
	OverriddenByVersion  string
	OverriddenByLabels   map[string]string
	DeployInstructionID  string
}

type ViewPrefs struct {
	Group          string // "tenant", "assignment", "version"
	ShowVersion    bool
	ShowLastDeploy bool
	ShowRowActions bool
}

func overviewViewPrefs() ViewPrefs {
	return ViewPrefs{
		Group:          "tenant",
		ShowVersion:    true,
		ShowLastDeploy: true,
		ShowRowActions: true,
	}
}

func assignmentSpecsViewPrefs() ViewPrefs {
	return ViewPrefs{
		Group:          "assignment",
		ShowVersion:    false,
		ShowLastDeploy: true,
		ShowRowActions: true,
	}
}

type card struct {
	Title               string
	LinkHref            string
	Labels              map[string]string
	FeatureAssignmentID string
	Environments        []AssignmentEnvStatus
}

func loadAssignmentData(ctx context.Context, feature *model.Feature, data *DetailPage) {
	data.AssignmentEnvs = featureAssignmentEnvStatuses(ctx, feature)
}

func assignmentDetailContent(data *DetailPage) g.Node {
	envs := currentAssignmentEnvStatuses(data.AssignmentEnvs)
	if len(envs) == 0 {
		return h.P(g.Text("No environments found."))
	}

	featureName := data.CurrentFeature.Name
	chart := data.CurrentFeature.Chart

	return h.Div(h.ID("env-overview"), g.Attr("data-view", "grid"),
		overviewTable(envs, featureName),
		overviewCardGrid(envs, featureName, chart),
	)
}

func overviewToolbar() g.Node {
	return h.Button(h.Type("button"), h.Class("view-toggle-btn"), h.ID("view-toggle"),
		g.Attr("data-view-toggle", "env-overview"),
		g.Attr("aria-label", "Toggle view"),
		g.Attr("title", "Toggle table/grid view"),
		g.Raw(`<svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><rect x="1" y="2" width="3" height="12" rx="0.5"/><rect x="6.5" y="2" width="3" height="12" rx="0.5"/><rect x="12" y="2" width="3" height="12" rx="0.5"/></svg>`),
	)
}

func overviewTable(envs []AssignmentEnvStatus, featureName string) g.Node {
	sortEnvs(envs)
	prefs := overviewViewPrefs()
	thNodes := []g.Node{
		h.Th(g.Text("Tenant")),
		h.Th(g.Text("Env")),
		h.Th(g.Text("Status")),
		h.Th(g.Text("Version")),
		h.Th(h.Title("When the latest successful deployment instruction completed"), g.Text("Deployed")),
		h.Th(h.Class("col-action"), g.Attr("data-no-sort", "")),
	}
	rows := g.Map(envs, func(env AssignmentEnvStatus) g.Node {
		return envCardRow(env, featureName, prefs, true, true, true, true)
	})
	table := h.Table(h.Class("table sortable"), g.Attr("data-sort-key", "feature-overview"),
		h.THead(h.Tr(g.Group(thNodes))),
		h.TBody(g.Group(rows)),
	)
	return h.Div(h.Class("feature-overview-table"), h.ID("overview-table"),
		h.Div(h.Class("feature-card"),
			h.Div(h.Class("feature-card-body"), table),
		),
	)
}

func overviewCardGrid(envs []AssignmentEnvStatus, featureName, chart string) g.Node {
	prefs := ViewPrefs{Group: "tenant", ShowVersion: false, ShowLastDeploy: false, ShowRowActions: false}
	cards := groupByTenantCards(envs)
	return h.Div(h.Class("feature-overview-grid"), h.ID("overview-grid"),
		cardGrid(cards, featureName, chart, prefs, nil),
	)
}

func currentAssignmentEnvStatuses(envs []AssignmentEnvStatus) []AssignmentEnvStatus {
	ret := make([]AssignmentEnvStatus, 0, len(envs))
	for _, env := range envs {
		if env.IsOverridden {
			continue
		}
		ret = append(ret, env)
	}
	return ret
}

func assignmentSpecsContent(data *DetailPage) g.Node {
	if len(data.AssignmentEnvs) == 0 {
		return h.P(g.Text("No environments found."))
	}

	prefs := assignmentSpecsViewPrefs()
	cards := buildCards(data.AssignmentEnvs, prefs.Group)
	fallbacks := fallbackVersionMap(data.AssignmentEnvs)
	return cardGrid(cards, data.CurrentFeature.Name, data.CurrentFeature.Chart, prefs, fallbacks)
}

// fallbackVersionMap returns a map from deployment ID to the version that
// would take over if that deployment is removed. This is determined by looking
// at environments that are overridden BY this deployment — their own deployment
// version is the fallback.
func fallbackVersionMap(envs []AssignmentEnvStatus) map[string]string {
	fallbacks := map[string]string{}
	for _, env := range envs {
		if env.IsOverridden && env.OverriddenByID != "" {
			fallbacks[env.OverriddenByID] = env.AssignmentVersion
		}
	}
	return fallbacks
}

func buildCards(envs []AssignmentEnvStatus, groupBy string) []card {
	switch groupBy {
	case "assignment":
		return groupByAssignmentCards(envs)
	case "version":
		return groupByVersionCards(envs)
	default:
		return groupByTenantCards(envs)
	}
}

func groupByTenantCards(envs []AssignmentEnvStatus) []card {
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

func groupByAssignmentCards(envs []AssignmentEnvStatus) []card {
	groups := map[string]*card{}
	var order []string
	for _, env := range envs {
		if _, ok := groups[env.FeatureAssignmentID]; !ok {
			groups[env.FeatureAssignmentID] = &card{
				Title:               env.AssignmentVersion,
				LinkHref:            "/assignments/" + env.FeatureAssignmentID,
				Labels:              env.TargetLabels,
				FeatureAssignmentID: env.FeatureAssignmentID,
			}
			order = append(order, env.FeatureAssignmentID)
		}
		groups[env.FeatureAssignmentID].Environments = append(groups[env.FeatureAssignmentID].Environments, env)
	}
	result := make([]card, 0, len(order))
	for _, id := range order {
		c := *groups[id]
		sortEnvs(c.Environments)
		result = append(result, c)
	}
	return result
}

func groupByVersionCards(envs []AssignmentEnvStatus) []card {
	groups := map[string]*card{}
	var order []string
	for _, env := range envs {
		version := env.AssignmentVersion
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

func sortEnvs(envs []AssignmentEnvStatus) {
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

func cardGrid(cards []card, featureName, chart string, prefs ViewPrefs, fallbackVersions map[string]string) g.Node {
	if len(cards) == 0 {
		return h.P(h.Class("text-muted"), g.Text("No environments to show."))
	}
	return h.Div(h.Class("feature-card-grid"),
		g.Map(cards, func(c card) g.Node {
			return renderCard(c, featureName, chart, prefs, fallbackVersions[c.FeatureAssignmentID])
		}),
	)
}

func renderCard(c card, featureName, chart string, prefs ViewPrefs, fallbackVersion string) g.Node {
	heading := []g.Node{}

	if prefs.Group == "tenant" {
		heading = append(heading,
			components.TenantAvatar(c.Title, components.HasTenantLogo(c.Title), "20px"),
		)
	}

	if c.LinkHref != "" {
		heading = append(heading,
			h.A(h.Href(c.LinkHref), h.Class("card-group-title-link"), g.Text(c.Title)),
		)
	} else {
		heading = append(heading,
			h.Span(h.Class("card-group-title"), g.Text(c.Title)),
		)
	}

	if prefs.Group == "assignment" || len(c.Labels) > 0 {
		heading = append(heading, h.Span(h.Class("feature-card-labels"), labelPills(c.Labels)))
	}

	actions := []g.Node{}
	// Card-level kebab for deployment grouping (set version, remove)
	if prefs.Group == "assignment" && c.FeatureAssignmentID != "" {
		setVersionPopoverID := "set-version-" + c.FeatureAssignmentID
		removePopoverID := "remove-assignment-" + c.FeatureAssignmentID
		actions = append(actions,
			h.Div(h.Class("card-kebab-wrap"),
				components.KebabButton("card-kebab-"+c.FeatureAssignmentID),
				h.Div(h.Class("kebab-menu"), h.ID("card-kebab-"+c.FeatureAssignmentID),
					h.Button(h.Type("button"), h.Class("kebab-item"),
						g.Attr("popovertarget", setVersionPopoverID),
						g.Text("Set version"),
					),
					h.Button(h.Type("button"), h.Class("kebab-item kebab-item-danger"),
						g.Attr("popovertarget", removePopoverID),
						g.Text("Remove"),
					),
				),
				setVersionPopover(setVersionPopoverID, featureName, chart, c.Labels),
				h.Div(g.Attr("popover", ""), h.ID(removePopoverID),
					h.H3(g.Text("Remove assignment spec")),
					g.If(fallbackVersion != "",
						h.P(g.Textf("This will remove this assignment spec. Version %s will take its place.", fallbackVersion)),
					),
					g.If(fallbackVersion == "",
						h.P(g.Text("This will remove this assignment spec. It will no longer be reconciled.")),
					),
					h.Form(h.Method("POST"), h.Action("/assignments/"+c.FeatureAssignmentID+"/deactivate"),
						h.Input(h.Type("hidden"), h.Name("redirect"), h.Value("/features/"+featureName+"/assignments")),
						h.Div(h.Class("popover-actions"),
							h.Button(h.Type("submit"), g.Text("Remove")),
							h.Button(h.Type("button"), g.Attr("popovertarget", removePopoverID), g.Attr("popovertargetaction", "hide"), g.Text("Cancel")),
						),
					),
				),
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
	thNodes = append(thNodes, h.Th(g.Text("Env")))
	thNodes = append(thNodes, h.Th(g.Text("Status")))
	if showVersion {
		thNodes = append(thNodes, h.Th(g.Text("Version")))
	}
	if prefs.ShowLastDeploy {
		thNodes = append(thNodes, h.Th(h.Title("When the latest successful deployment instruction completed"), g.Text("Deployed")))
	}
	if prefs.ShowRowActions {
		thNodes = append(thNodes, h.Th(h.Class("col-action"), g.Attr("data-no-sort", "")))
	}

	rows := g.Map(c.Environments, func(env AssignmentEnvStatus) g.Node {
		return envCardRow(env, featureName, prefs, showTenant, showVersion, false, prefs.ShowRowActions)
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

func envCardRow(env AssignmentEnvStatus, featureName string, prefs ViewPrefs, showTenant, showVersion, showTenantAvatar, showActions bool) g.Node {
	baseHref := "/features/" + featureName + "/envs/" + env.TenantSlug + "/" + env.Name
	envLink := h.A(h.Href(baseHref), g.Text(env.Name))
	logsHref := baseHref
	if env.DeployInstructionID != "" {
		logsHref += "?logs=" + env.DeployInstructionID
	}

	rowAttrs := []g.Node{}
	if env.IsOverridden {
		rowAttrs = append(rowAttrs, h.Class("assignment-overridden"))
	}

	cells := []g.Node{}
	if showTenant {
		cells = append(cells, tenantCell(env, showTenantAvatar))
	}

	hasDrift := env.ReleaseVersion != "" && env.ReleaseVersion != env.AssignmentVersion
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
	statusCell = append(statusCell, components.Status(env.StatusText))
	cells = append(cells, h.Td(statusCell...))

	if showVersion {
		cells = append(cells, h.Td(g.Text(env.AssignmentVersion), driftIcon))
	}

	if prefs.ShowLastDeploy {
		cells = append(cells, lastDeployedCell(env.LastDeployed, ""))
	}

	if showActions {
		kebabID := "row-kebab-" + env.TenantSlug + "-" + env.Name
		redeployPopoverID := "redeploy-" + env.TenantSlug + "-" + env.Name
		reconcilePopoverID := "reconcile-" + env.TenantSlug + "-" + env.Name
		redeployAction := baseHref + "/redeploy"
		toggleReconcileAction := baseHref + "/toggle-reconcile"

		menuItems := []g.Node{
			h.A(h.Href(logsHref), h.Class("kebab-item"),
				g.Raw(components.IconLogs),
				g.Text("Deploy logs"),
			),
			components.LokiLogsItem(environment.LokiExploreURL(env.TenantName, env.Name, featureName)),
		}

		if env.Enabled {
			menuItems = append(menuItems,
				h.Button(h.Type("button"), h.Class("kebab-item"), g.Attr("popovertarget", redeployPopoverID),
					g.Raw(components.IconRedeploy),
					g.Text("Trigger redeploy"),
				),
			)
		}

		if env.Enabled {
			menuItems = append(menuItems,
				h.Button(h.Type("button"), h.Class("kebab-item kebab-item-danger"), g.Attr("popovertarget", reconcilePopoverID),
					g.Raw(components.IconPause),
					g.Text("Disable reconcile"),
				),
			)
		} else {
			menuItems = append(menuItems,
				h.Button(h.Type("button"), h.Class("kebab-item"), g.Attr("popovertarget", reconcilePopoverID),
					g.Raw(components.IconPlay),
					g.Text("Enable reconcile"),
				),
			)
		}

		menuItems = append(menuItems,
			h.A(h.Href("/assignments/"+env.FeatureAssignmentID), h.Class("kebab-item"),
				g.Raw(components.IconDocument),
				g.Text("View assignment"),
			),
		)

		cells = append(cells, h.Td(h.Class("action"),
			components.KebabWrap(kebabID, menuItems,
				components.RedeployPopover(redeployPopoverID, redeployAction, featureName, env.Name, env.Enabled, "/features/"+featureName),
				components.ReconcilePopover(reconcilePopoverID, toggleReconcileAction, featureName, env.Name, env.Enabled, "/features/"+featureName),
			),
		))
	}

	return h.Tr(g.Group(append(rowAttrs, cells...)))
}

func tenantCell(env AssignmentEnvStatus, showAvatar bool) g.Node {
	if !showAvatar {
		return h.Td(g.Text(env.TenantName))
	}
	return h.Td(h.Span(h.Class("tenant-cell"),
		components.TenantAvatar(env.TenantName, components.HasTenantLogo(env.TenantName), "20px"),
		h.Span(g.Text(env.TenantName)),
	))
}

func statusTooltip(env AssignmentEnvStatus) string {
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
		if env.DisableReason != "" {
			tip += ": " + env.DisableReason
		} else {
			tip += ": disabled before we started requiring reason"
		}
		if env.ReleaseVersion != "" {
			tip += " — Running: " + env.ReleaseVersion
		}
		return tip
	}
	if env.ReleaseVersion != "" && env.ReleaseVersion != env.AssignmentVersion {
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
		h.Form(h.Method("POST"), h.Action("/assignments"),
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

func featureAssignmentEnvStatuses(ctx context.Context, feature *model.Feature) []AssignmentEnvStatus {
	fas, err := featureassignment.ListByFeature(ctx, feature.Name)
	if err != nil || len(fas) == 0 {
		return []AssignmentEnvStatus{}
	}
	fas = latestAssignmentPerTarget(fas)

	tenants, err := uidata.ListTenants(ctx)
	if err != nil {
		return []AssignmentEnvStatus{}
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

	winners := map[uuid.UUID]*featureassignment.FeatureAssignment{}
	for _, env := range allEnvs {
		for _, fa := range fas {
			if !targetMatchesLabels(fa.TargetLabels, env.labels) {
				continue
			}
			existing, ok := winners[env.env.ID]
			if !ok || featureassignment.IsMoreSpecific(fa.TargetLabels, existing.TargetLabels, fa.Created, existing.Created) {
				winners[env.env.ID] = fa
			}
		}
	}

	statusByAssignmentEnv := map[string]*featureassignment.FeatureReconcileStatus{}
	for _, fa := range fas {
		statuses, err := featureassignment.ListFeatureReconcileStatuses(ctx, fa.ID)
		if err != nil {
			continue
		}
		for _, status := range statuses {
			statusByAssignmentEnv[fa.ID.String()+":"+status.EnvironmentID.String()] = status
		}
	}

	ret := []AssignmentEnvStatus{}
	for _, fa := range fas {
		for _, env := range allEnvs {
			if !targetMatchesLabels(fa.TargetLabels, env.labels) {
				continue
			}

			disabledAt, disabled, err := featurepkg.FeatureDisabledAt(ctx, env.env.ID, feature.Name)
			if err != nil {
				continue
			}

			es := AssignmentEnvStatus{
				Name:                 env.env.Name,
				TenantName:           env.tenantName,
				TenantSlug:           env.tenantName,
				Enabled:              !disabled,
				EnvReconcileDisabled: !env.env.Reconcile,
				FeatureAssignmentID:  fa.ID.String(),
				AssignmentVersion:    fa.Feature.Version,
				ChartDescription:     fa.Feature.Description,
				TargetLabels:         fa.TargetLabels,
			}
			if disabled {
				es.LastModified = disabledAt
				es.DisableReason = audit.LatestDisableReason(ctx, feature.Name, env.env.ID)
			}

			if di, err := featurepkg.GetLatestDeployedDeployInstruction(ctx, env.env.ID, feature.Name); err == nil && di != nil {
				es.LastDeployed = di.LastModified
			}
			if di, err := featurepkg.GetLatestDeployInstruction(ctx, env.env.ID, feature.Name); err == nil && di != nil {
				es.DeployInstructionID = di.ID.String()
			}

			releases, err := featureassignment.ListReleaseStatuses(ctx, env.env.ID)
			if err == nil {
				for _, release := range releases {
					if release.Name == feature.Name {
						es.ReleaseVersion = release.Version
						break
					}
				}
			}

			winner := winners[env.env.ID]
			if winner != nil && winner.ID != fa.ID {
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
				if status := statusByAssignmentEnv[fa.ID.String()+":"+env.env.ID.String()]; status != nil {
					es.StatusText = featureassignment.NormalizeStatus(string(status.State))
					es.LastModified = status.LastModified
				}
			}

			ret = append(ret, es)
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
