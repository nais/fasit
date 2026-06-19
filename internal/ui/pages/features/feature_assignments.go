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
	"github.com/nais/fasit/internal/naisdstatus"
	"github.com/nais/fasit/internal/reconciler"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/pages/environment"
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
	StatusTime     bool // time column shows when the current status was produced, not last deploy
}

func overviewViewPrefs() ViewPrefs {
	return ViewPrefs{
		Group:          "tenant",
		ShowVersion:    true,
		ShowLastDeploy: true,
		ShowRowActions: true,
		StatusTime:     true,
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

func loadAssignmentData(ctx context.Context, feature *featurepkg.Feature, data *DetailPage) {
	data.AssignmentEnvs = featureAssignmentEnvStatuses(ctx, feature)
}

func assignmentDetailContent(data *DetailPage) g.Node {
	envs := currentAssignmentEnvStatuses(data.AssignmentEnvs)
	if len(envs) == 0 {
		return h.P(g.Text("No environments found."))
	}

	featureName := data.CurrentFeature.Name

	return h.Div(h.ID("env-overview"),
		overviewTable(envs, featureName),
	)
}

func overviewTable(envs []AssignmentEnvStatus, featureName string) g.Node {
	sortEnvs(envs)
	prefs := overviewViewPrefs()
	thNodes := []g.Node{
		h.Th(g.Text("Tenant")),
		h.Th(g.Text("Env")),
		h.Th(g.Text("Status")),
		h.Th(h.Title("When the current status was last updated"), g.Text("When")),
		h.Th(g.Text("Version")),
		h.Th(h.Class("col-action"), g.Attr("data-no-sort", "")),
	}
	versionEmph := assignmentVersionEmphasis(envs)
	rows := make([]g.Node, len(envs))
	for i, env := range envs {
		rows[i] = envCardRow(env, featureName, prefs, true, true, true, true, versionEmph[i])
	}
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
	cards := groupByAssignmentCards(data.AssignmentEnvs)
	fallbacks := fallbackVersionMap(data.AssignmentEnvs)
	return cardGrid(cards, data.CurrentFeature.Name, data.CurrentFeature.Chart, prefs, fallbacks)
}

// fallbackVersionMap returns a map from assignment ID to the version that
// would take over if that assignment is removed. This is determined by looking
// at environments that are overridden BY this assignment — their own assignment
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
				components.Popover(removePopoverID, "", "Remove assignment spec",
					g.If(fallbackVersion != "",
						h.P(g.Textf("This will remove this assignment spec. Version %s will take its place.", fallbackVersion)),
					),
					g.If(fallbackVersion == "",
						h.P(g.Text("This will remove this assignment spec. It will no longer be reconciled.")),
					),
					h.Form(h.Method("POST"), h.Action("/assignments/"+c.FeatureAssignmentID+"/deactivate"),
						h.Input(h.Type("hidden"), h.Name("redirect"), h.Value("/features/"+featureName+"/assignments")),
						components.PopoverActions(
							h.Button(h.Type("submit"), g.Text("Remove")),
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

	versionEmph := assignmentVersionEmphasis(c.Environments)
	rows := make([]g.Node, len(c.Environments))
	for i, env := range c.Environments {
		rows[i] = envCardRow(env, featureName, prefs, showTenant, showVersion, false, prefs.ShowRowActions, versionEmph[i])
	}

	table := h.Table(h.Class("table sortable"), g.Attr("data-sort-key", "feature-card"),
		h.THead(h.Tr(g.Group(thNodes))),
		h.TBody(g.Group(rows)),
	)

	return h.Div(h.Class("feature-card"),
		header,
		h.Div(h.Class("feature-card-body"), table),
	)
}

func assignmentVersionEmphasis(envs []AssignmentEnvStatus) []components.Emphasis {
	versions := make([]string, len(envs))
	for i, env := range envs {
		versions[i] = env.AssignmentVersion
	}
	return components.ColumnConsensus(versions)
}

func envCardRow(env AssignmentEnvStatus, featureName string, prefs ViewPrefs, showTenant, showVersion, showTenantAvatar, showActions bool, versionEmph components.Emphasis) g.Node {
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

	if prefs.StatusTime {
		cells = append(cells, lastDeployedCell(env.LastModified, ""))
	}

	if showVersion {
		cells = append(cells, h.Td(components.ConsensusCell(versionEmph, g.Text(env.AssignmentVersion)), driftIcon))
	}

	if prefs.ShowLastDeploy && !prefs.StatusTime {
		cells = append(cells, lastDeployedCell(env.LastDeployed, ""))
	}

	if showActions {
		kebabID := "row-kebab-" + env.TenantSlug + "-" + env.Name
		reconcilePopoverID := "reconcile-" + env.TenantSlug + "-" + env.Name
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
	switch env.StatusText {
	case "MISSING-DEPS", "MISSING-CONFIG", "RENDER-ERROR", "FAILED":
		return ""
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
	return components.Popover(popoverID, "", "Set version",
		h.Form(h.Method("POST"), h.Action("/assignments"),
			g.Group(inputs),
			h.Label(g.Text("Version")),
			h.Input(h.Type("text"), h.Name("version"), g.Attr("required", ""), g.Attr("autofocus", "")),
			components.PopoverActions(
				h.Button(h.Type("submit"), g.Text("Set version")),
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

func featureAssignmentEnvStatuses(ctx context.Context, feature *featurepkg.Feature) []AssignmentEnvStatus {
	fas, err := featureassignment.ListByFeature(ctx, feature.Name)
	if err != nil || len(fas) == 0 {
		return []AssignmentEnvStatus{}
	}
	fas = latestAssignmentPerTarget(fas)

	tenantEnvs, err := envpkg.ListTenantEnvironments(ctx, false)
	if err != nil {
		return []AssignmentEnvStatus{}
	}

	type envInfo struct {
		env        *envpkg.Environment
		tenantName string
		labels     map[string]string
	}
	var allEnvs []envInfo
	for _, te := range tenantEnvs {
		if !featureTargetsKind(feature.EnvironmentKinds, te.Kind) {
			continue
		}
		env := te.Environment
		allEnvs = append(allEnvs, envInfo{env: &env, tenantName: te.TenantName, labels: te.Labels})
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

	statusesByFA, err := reconciler.AllReconcileStatuses(ctx)
	if err != nil {
		return []AssignmentEnvStatus{}
	}
	statusByAssignmentEnv := map[string]*reconciler.FeatureReconcileStatus{}
	for _, fa := range fas {
		for _, status := range statusesByFA[fa.ID] {
			statusByAssignmentEnv[fa.ID.String()+":"+status.EnvironmentID.String()] = status
		}
	}

	disabledByEnv, err := featurepkg.DisabledEnvironmentsForFeature(ctx, feature.Name)
	if err != nil {
		return []AssignmentEnvStatus{}
	}
	latestDeployed, err := featurepkg.LatestDeployedForFeature(ctx, feature.Name)
	if err != nil {
		return []AssignmentEnvStatus{}
	}
	latestInstruction, err := featurepkg.LatestDeployInstructionsForFeature(ctx, feature.Name)
	if err != nil {
		return []AssignmentEnvStatus{}
	}
	releaseByEnv, err := naisdstatus.ReleaseStatusesForFeature(ctx, feature.Name)
	if err != nil {
		return []AssignmentEnvStatus{}
	}

	ret := []AssignmentEnvStatus{}
	for _, fa := range fas {
		for _, env := range allEnvs {
			if !targetMatchesLabels(fa.TargetLabels, env.labels) {
				continue
			}

			disabledAt, disabled := disabledByEnv[env.env.ID]

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

			if di := latestDeployed[env.env.ID]; di != nil {
				es.LastDeployed = di.LastModified
			}
			if di := latestInstruction[env.env.ID]; di != nil {
				es.DeployInstructionID = di.ID.String()
			}

			if release := releaseByEnv[env.env.ID]; release != nil {
				es.ReleaseVersion = release.Version
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
					es.StatusText = reconciler.NormalizeStatus(string(status.State))
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
