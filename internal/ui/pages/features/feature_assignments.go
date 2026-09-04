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
	"github.com/nais/fasit/internal/ui/uidata"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type AssignmentEnvStatus struct {
	Name                  string
	EnvironmentID         string
	TenantName            string
	TenantSlug            string
	Enabled               bool
	DisableReason         string
	EnvReconcileDisabled  bool
	LastModified          time.Time
	LastDeployed          time.Time
	StatusText            string
	FeatureAssignmentID   string
	AssignmentVersion     string
	AssignmentDescription string
	ChartDescription      string
	ReleaseVersion        string
	TargetLabels          map[string]string
	IsOverridden          bool
	OverriddenByID        string
	OverriddenByVersion   string
	OverriddenByLabels    map[string]string
	DeployInstructionID   string
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
	Creator             string
	Description         string
	Environments        []AssignmentEnvStatus
}

type assignmentLabelOption struct {
	Key    string
	Values []string
}

func assignmentCreators(ctx context.Context, envs []AssignmentEnvStatus) (map[string]string, error) {
	seen := make(map[uuid.UUID]struct{})
	ids := make([]uuid.UUID, 0)
	for _, env := range envs {
		id, err := uuid.Parse(env.FeatureAssignmentID)
		if err != nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	creators, err := audit.AssignmentCreators(ctx, ids)
	if err != nil {
		return nil, err
	}
	ret := make(map[string]string, len(creators))
	for id, creator := range creators {
		ret[id.String()] = creator
	}
	return ret, nil
}

func knownAssignmentVersions(ctx context.Context, feature *featurepkg.Feature) ([]string, error) {
	versions, err := uidata.FeatureVersions(ctx, feature.Name)
	if err != nil {
		return nil, err
	}
	ret := make([]string, 0, len(versions)+1)
	ret = append(ret, feature.Version)
	for _, version := range versions {
		ret = append(ret, version.Version)
	}
	return mergeVersions(ret), nil
}

func mergeVersions(versionSets ...[]string) []string {
	seen := make(map[string]struct{})
	var ret []string
	for _, versions := range versionSets {
		for _, version := range versions {
			if version == "" {
				continue
			}
			if _, ok := seen[version]; ok {
				continue
			}
			seen[version] = struct{}{}
			ret = append(ret, version)
		}
	}
	return ret
}

func loadAssignmentLabelOptions(ctx context.Context, feature *featurepkg.Feature) ([]assignmentLabelOption, error) {
	tenantEnvs, err := envpkg.ListTenantEnvironments(ctx, false)
	if err != nil {
		return nil, err
	}
	valuesByKey := make(map[string]map[string]struct{})
	for _, tenantEnv := range tenantEnvs {
		if !featureTargetsKind(feature.EnvironmentKinds, tenantEnv.Kind) {
			continue
		}
		for key, value := range tenantEnv.Labels {
			if valuesByKey[key] == nil {
				valuesByKey[key] = make(map[string]struct{})
			}
			valuesByKey[key][value] = struct{}{}
		}
	}
	keys := make([]string, 0, len(valuesByKey))
	for key := range valuesByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ret := make([]assignmentLabelOption, 0, len(keys))
	for _, key := range keys {
		values := make([]string, 0, len(valuesByKey[key]))
		for value := range valuesByKey[key] {
			values = append(values, value)
		}
		sort.Strings(values)
		ret = append(ret, assignmentLabelOption{Key: key, Values: values})
	}
	return ret, nil
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

	return h.Div(
		h.ID("env-overview"),
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
	table := h.Table(
		h.Class("table sortable"), g.Attr("data-sort-key", "feature-overview"),
		h.THead(h.Tr(g.Group(thNodes))),
		h.TBody(g.Group(rows)),
	)
	return h.Div(
		h.Class("feature-overview-table"), h.ID("overview-table"),
		h.Div(
			h.Class("feature-card"),
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
	featureName := data.CurrentFeature.Name
	content := []g.Node{
		h.Div(
			h.Class("assignments-header"),
			h.Div(h.Class("assignments-toolbar"), h.H2(g.Text("Assignments"))),
			h.Button(h.Type("button"), h.Class("btn-small"), g.Attr("popovertarget", "new-feature-assignment"), g.Text("+ New assignment")),
			newFeatureAssignmentPopover(data),
		),
	}

	if len(data.AssignmentEnvs) == 0 {
		content = append(content, h.P(h.Class("text-muted"), g.Text("No assignments found.")))
		return h.Div(g.Group(content))
	}

	prefs := assignmentSpecsViewPrefs()
	cards := groupByAssignmentCards(data.AssignmentEnvs, featureName, data.AssignmentCreators)
	fallbacks := fallbackVersionMap(data.AssignmentEnvs)
	content = append(content, cardGrid(cards, featureName, data.CurrentFeature.Chart, prefs, fallbacks))
	return h.Div(g.Group(content))
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

func groupByAssignmentCards(envs []AssignmentEnvStatus, featureName string, creators map[string]string) []card {
	groups := map[string]*card{}
	var order []string
	for _, env := range envs {
		if _, ok := groups[env.FeatureAssignmentID]; !ok {
			groups[env.FeatureAssignmentID] = &card{
				Title:               env.AssignmentVersion,
				LinkHref:            "/features/" + featureName + "/assignments/" + env.FeatureAssignmentID,
				Labels:              env.TargetLabels,
				FeatureAssignmentID: env.FeatureAssignmentID,
				Creator:             creators[env.FeatureAssignmentID],
				Description:         env.AssignmentDescription,
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
	return h.Div(
		h.Class("assignment-list"),
		g.Map(cards, func(c card) g.Node {
			return renderCard(c, featureName, chart, prefs, fallbackVersions[c.FeatureAssignmentID])
		}),
	)
}

func renderCard(c card, featureName, chart string, prefs ViewPrefs, fallbackVersion string) g.Node {
	head := h.Div(
		h.Class("assignment-row-head"),
		h.Span(h.Class("card-group-title"), g.Text(c.Title)),
		flowArrow(),
		h.Span(h.Class("feature-card-labels"), labelPills(c.Labels)),
	)

	content := []g.Node{head}
	if description := strings.TrimSpace(c.Description); description != "" && description != "Set via UI" {
		content = append(content, h.Span(h.Class("assignment-description"), h.Title(description), g.Text(description)))
	}
	content = append(content, assignmentStatusSummary(c.Environments))

	var main g.Node
	if c.LinkHref != "" {
		main = h.A(
			h.Href(c.LinkHref), h.Class("assignment-row-link"),
			g.Group(content),
		)
	} else {
		main = h.Div(
			h.Class("assignment-row-link"),
			g.Group(content),
		)
	}

	var actions g.Node
	if c.FeatureAssignmentID != "" {
		setVersionPopoverID := "set-version-" + c.FeatureAssignmentID
		removePopoverID := "remove-assignment-" + c.FeatureAssignmentID
		actions = h.Div(
			h.Class("feature-card-actions"),
			h.Div(
				h.Class("card-kebab-wrap"),
				components.KebabButton("card-kebab-"+c.FeatureAssignmentID),
				h.Div(
					h.Class("kebab-menu"), h.ID("card-kebab-"+c.FeatureAssignmentID),
					h.Button(
						h.Type("button"), h.Class("kebab-item"),
						g.Attr("popovertarget", setVersionPopoverID),
						g.Text("Set version"),
					),
					h.Button(
						h.Type("button"), h.Class("kebab-item kebab-item-danger"),
						g.Attr("popovertarget", removePopoverID),
						g.Text("Remove"),
					),
				),
				setVersionPopover(setVersionPopoverID, featureName, chart, c.Labels),
				components.Popover(
					removePopoverID, "", "Remove assignment",
					g.If(
						fallbackVersion != "",
						h.P(g.Textf("This will remove this assignment. Version %s will take its place.", fallbackVersion)),
					),
					g.If(
						fallbackVersion == "",
						h.P(g.Text("This will remove this assignment. It will no longer be reconciled.")),
					),
					h.Form(
						h.Method("POST"), h.Action("/assignments/"+c.FeatureAssignmentID+"/remove"),
						h.Input(h.Type("hidden"), h.Name("redirect"), h.Value("/features/"+featureName+"/assignments")),
						components.PopoverActions(
							h.Button(h.Type("submit"), g.Text("Remove")),
						),
					),
				),
			),
		)
	}

	rowClass := "assignment-row"
	if !view.IsWorkflowActor(c.Creator) {
		rowClass += " assignment-non-workflow"
	}
	return h.Div(
		h.Class(rowClass),
		h.Div(
			h.Class("assignment-row-main"),
			main,
			h.Div(h.Class("assignment-creator"), g.Text("Created by "), view.AssignmentCreatorNode(c.Creator)),
		),
		actions,
	)
}

func flowArrow() g.Node {
	return h.Span(h.Class("assignment-flow-arrow"), g.Text("→"))
}

func assignmentStatusSummary(envs []AssignmentEnvStatus) g.Node {
	counts := map[string]int{}
	for _, e := range envs {
		counts[e.StatusText]++
	}

	order := []string{
		"DEPLOYED", "INSTALLING", "SENT", "PENDING", "UNKNOWN",
		"MISSING-DEPS", "MISSING-CONFIG", "RENDER-ERROR", "FAILED", "UNHEALTHY",
		"DISABLED", "OVERRIDDEN", "INACTIVE",
	}
	seen := map[string]bool{}
	var statuses []string
	for _, s := range order {
		if counts[s] > 0 {
			statuses = append(statuses, s)
			seen[s] = true
		}
	}
	var rest []string
	for s := range counts {
		if !seen[s] {
			rest = append(rest, s)
		}
	}
	sort.Strings(rest)
	statuses = append(statuses, rest...)

	items := make([]g.Node, len(statuses))
	for i, s := range statuses {
		items[i] = h.Span(
			h.Class("assignment-status"),
			h.Span(h.Class("status-dot "+components.StatusClass(s))),
			g.Textf("%d %s", counts[s], statusLabel(s)),
		)
	}
	return h.Div(h.Class("assignment-summary"), g.Group(items))
}

func statusLabel(status string) string {
	switch status {
	case "MISSING-DEPS":
		return "Missing deps"
	case "MISSING-CONFIG":
		return "Missing config"
	case "RENDER-ERROR":
		return "Render error"
	default:
		return strings.ToUpper(status[:1]) + strings.ToLower(status[1:])
	}
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
	driftIcon := g.If(
		hasDrift,
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
		cells = append(cells, view.TimeCell(env.LastModified))
	}

	if showVersion {
		cells = append(cells, h.Td(components.ConsensusCell(versionEmph, g.Text(env.AssignmentVersion)), driftIcon))
	}

	if prefs.ShowLastDeploy && !prefs.StatusTime {
		cells = append(cells, view.TimeCell(env.LastDeployed))
	}

	if showActions {
		kebabID := "row-kebab-" + env.TenantSlug + "-" + env.Name
		reconcilePopoverID := "reconcile-" + env.TenantSlug + "-" + env.Name
		toggleReconcileAction := baseHref + "/toggle-reconcile"

		menuItems := []g.Node{
			h.A(
				h.Href(logsHref), h.Class("kebab-item"),
				g.Raw(components.IconLogs),
				g.Text("Deploy logs"),
			),
			components.LokiLogsItem(environment.LokiExploreURL(env.TenantName, env.Name, featureName)),
		}

		if env.Enabled {
			menuItems = append(
				menuItems,
				h.Button(
					h.Type("button"), h.Class("kebab-item kebab-item-danger"), g.Attr("popovertarget", reconcilePopoverID),
					g.Raw(components.IconPause),
					g.Text("Disable reconcile"),
				),
			)
		} else {
			menuItems = append(
				menuItems,
				h.Button(
					h.Type("button"), h.Class("kebab-item"), g.Attr("popovertarget", reconcilePopoverID),
					g.Raw(components.IconPlay),
					g.Text("Enable reconcile"),
				),
			)
		}

		menuItems = append(
			menuItems,
			h.A(
				h.Href("/features/"+featureName+"/assignments/"+env.FeatureAssignmentID), h.Class("kebab-item"),
				g.Raw(components.IconDocument),
				g.Text("View assignment"),
			),
		)

		cells = append(cells, h.Td(
			h.Class("action"),
			components.KebabWrap(
				kebabID, menuItems,
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
	return h.Td(h.Span(
		h.Class("tenant-cell"),
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

func newFeatureAssignmentPopover(data *DetailPage) g.Node {
	versionOptions := g.Map(data.AssignmentVersions, func(version string) g.Node {
		return h.Option(h.Value(version), g.Text(version))
	})
	kindInputs := g.Map(data.CurrentFeature.EnvironmentKinds, func(kind envpkg.EnvironmentKind) g.Node {
		return h.Input(h.Type("hidden"), h.Name("environment_kind"), h.Value(string(kind)))
	})
	labelOptions := g.Map(data.AssignmentLabelOptions, func(option assignmentLabelOption) g.Node {
		values := g.Map(option.Values, func(value string) g.Node {
			return h.Span(g.Attr("data-label-value", ""), g.Text(value))
		})
		return h.Div(g.Attr("data-label-key", option.Key), g.Group(values))
	})

	return components.Popover(
		"new-feature-assignment", "assignment-popover", "New assignment",
		h.Form(
			h.Method("POST"), h.Action("/assignments"), g.Attr("data-assignment-form", ""),
			h.Input(h.Type("hidden"), h.Name("chart"), h.Value(data.CurrentFeature.Chart)),
			g.Group(kindInputs),
			h.Label(h.ID("assignment-version-label"), g.Text("Version")),
			h.Select(
				h.ID("assignment-version"), h.Name("version"), g.Attr("aria-labelledby", "assignment-version-label"), g.Attr("required", ""), g.Attr("data-version-select", ""),
				h.Option(h.Value(""), g.Attr("selected", ""), g.Attr("disabled", ""), g.Text("Choose a version…")),
				g.Group(versionOptions),
				h.Option(h.Value("__custom__"), g.Text("Enter another version…")),
			),
			h.Div(
				h.Class("assignment-custom-version"), g.Attr("data-custom-version", ""), g.Attr("hidden", ""),
				h.Input(h.ID("assignment-custom-version"), h.Type("text"), h.Name("version_custom"), g.Attr("aria-labelledby", "assignment-version-label"), g.Attr("autocomplete", "off"), h.Placeholder("Enter chart version")),
				h.Button(h.Type("button"), h.Class("btn-small btn-outline"), g.Attr("data-use-version-list", ""), g.Text("Use version list")),
			),
			h.P(h.Class("form-hint"), g.Text("Available versions are loaded from the chart registry.")),
			h.Label(g.Text("Description (optional)")),
			h.Input(h.Type("text"), h.Name("description"), h.Placeholder("e.g. Rollback to stable")),
			h.FieldSet(
				h.Class("assignment-label-builder"), g.Attr("data-label-builder", ""),
				h.Legend(g.Text("Target labels")),
				h.P(h.Class("form-hint"), g.Text("Add labels to narrow the target. No labels targets all environments.")),
				h.Div(g.Attr("data-label-options", ""), g.Attr("hidden", ""), g.Group(labelOptions)),
				h.Div(
					h.Class("assignment-label-rows"), g.Attr("data-label-rows", ""),
					h.Button(h.Type("button"), h.Class("assignment-add-label"), g.Attr("data-add-label", ""), g.Text("+ Add label")),
				),
			),
			h.Div(
				h.Class("assignment-target-preview"),
				h.Div(h.Class("assignment-target-preview-label"), g.Text("Target environments")),
				h.Div(h.ID("preview-targets-result"), h.Class("preview-targets-result"), g.Attr("aria-live", "polite")),
			),
			components.PopoverActions(h.Button(h.Type("submit"), g.Text("Create assignment"))),
		),
	)
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
	return components.Popover(
		popoverID, "", "Set version",
		h.Form(
			h.Method("POST"), h.Action("/assignments"),
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

	disabledByEnv, err := featurepkg.ListDisabledEnvironmentsForFeature(ctx, feature.Name)
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
				EnvironmentID:        env.env.ID.String(),
				TenantName:           env.tenantName,
				TenantSlug:           env.tenantName,
				Enabled:              !disabled,
				EnvReconcileDisabled: !env.env.Reconcile,
				FeatureAssignmentID:  fa.ID.String(),
				AssignmentVersion:    fa.Feature.Version,
				ChartDescription:     fa.Feature.Description,
				TargetLabels:         fa.TargetLabels,
			}
			if fa.Description != nil {
				es.AssignmentDescription = *fa.Description
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
