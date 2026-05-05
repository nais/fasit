package features

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/deployment"
	envpkg "github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/ui/breadcrumb"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, *http.Request, layout.Props)

type DetailPage struct {
	Breadcrumbs    []breadcrumb.Crumb
	Features       []view.FeatureNav
	CurrentFeature *Feature
	Environments   []EnvironmentStatus
	Rollouts       []RolloutItem
	Deployments    []DeploymentItem
	ActiveTab      string
	ShowAll        bool
}

type EnvironmentStatus struct {
	Name              string
	TenantName        string
	TenantSlug        string
	Enabled           bool
	LastModified      string
	StatusText        string
	HasDeployments    bool
	FeatureDeployable bool
	DeploymentID      string
	DeploymentVersion string
	ReleaseVersion    string
	TargetLabels      map[string]string
}

type Feature struct {
	*model.Feature
	Config []ConfigItem
}

type ConfigItem struct {
	Key        string
	Value      string
	Type       string
	IsSecret   bool
	IsComputed bool
	Template   string
}

type RolloutItem struct {
	FeatureName string
	Version     string
	Status      string
	Created     string
	Completed   string
	Target      string
}

type DeploymentItem struct {
	ID      string
	Version string
	Status  string
	Target  string
	Created string
}

func ListHandler(renderPage RenderPage, _ database.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		features, err := featurepkg.Features(r.Context())
		if err != nil {
			http.Error(w, "Failed to load features", http.StatusInternalServerError)
			return
		}
		renderPage(w, r, layout.Props{Title: "Features", CurrentPage: components.PageFeatures, Content: listPage(toFeatureNavs(features))})
	}
}

func TabHandler(renderPage RenderPage, repo database.Repo, activeTab string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := loadFeatureData(r, repo, activeTab)
		if err != nil {
			http.Error(w, "Failed to load feature data", http.StatusInternalServerError)
			return
		}
		renderPage(w, r, layout.Props{Title: data.CurrentFeature.Name, CurrentPage: components.PageFeatures, Content: detailPage(data)})
	}
}

func listPage(features []view.FeatureNav) g.Node {
	return h.Div(h.Class("container"), components.FeaturesSidebar(features, ""), h.Main(h.Class("main-content"), components.Breadcrumbs([]breadcrumb.Crumb{breadcrumb.Features()})))
}

func detailPage(data *DetailPage) g.Node {
	return h.Div(h.Class("container"), components.FeaturesSidebar(data.Features, data.CurrentFeature.Name), h.Main(h.Class("main-content"), components.Breadcrumbs(data.Breadcrumbs), h.Div(h.Class("card"), h.Div(h.Class("card-body"), components.TabsNav(data.ActiveTab, featureTabs(data.CurrentFeature.Name)), h.Div(h.Class("tab-content-wrapper"), tabContent(data))))))
}

func featureTabs(featureName string) []components.Tab {
	base := "/features/" + featureName
	return []components.Tab{{ID: "overview", Href: base, Label: "Overview"}, {ID: "status", Href: base + "/status", Label: "Status"}, {ID: "deployments", Href: base + "/deployments", Label: "Deployments"}, {ID: "rollouts", Href: base + "/rollouts", Label: "Rollouts"}}
}

func tabContent(data *DetailPage) g.Node {
	switch data.ActiveTab {
	case "status":
		return statusTab(data)
	case "deployments":
		return deploymentsTab(data)
	case "rollouts":
		return rolloutsTab(data)
	default:
		return overviewTab(data)
	}
}

func overviewTab(data *DetailPage) g.Node {
	return g.Group([]g.Node{h.Pre(g.Text(data.CurrentFeature.Description+"\n"), g.Text("chart: "+data.CurrentFeature.Chart+"\n"), g.Text("version: "+data.CurrentFeature.Version+"\n"), g.Text("source: "), h.A(h.Href(data.CurrentFeature.Source), h.Target("_blank"), g.Text(data.CurrentFeature.Source)), g.Text("\n"), g.Text("dependencies:"), dependencyLinks(data.CurrentFeature)), h.Table(h.Class("table"), h.THead(h.Tr(h.Th(g.Text("Key")), h.Th(g.Text("Type")), h.Th(g.Text("Default Value")))), h.TBody(g.Group(g.Map(data.CurrentFeature.Config, func(item ConfigItem) g.Node {
		return h.Tr(h.Td(h.Span(h.Class("icon"), g.Text("📄")), g.Text(" "), g.Text(item.Key)), h.Td(g.Text(configTypeLabel(item))), h.Td(h.Class("italic"), configDefaultValue(item)))
	}))))})
}

func statusTab(data *DetailPage) g.Node {
	featureName := data.CurrentFeature.Name
	var toggleLink g.Node
	if data.ShowAll {
		toggleLink = h.A(h.Href("/features/"+featureName+"/status"), h.Class("btn-small"), g.Text("Show enabled only"))
	} else {
		toggleLink = h.A(h.Href("/features/"+featureName+"/status?show=all"), h.Class("btn-small"), g.Text("Show all environments"))
	}
	if len(data.Environments) == 0 {
		return g.Group([]g.Node{
			h.Div(h.Class("table-actions"), toggleLink),
			h.P(g.Text("No environments found.")),
		})
	}

	if !data.CurrentFeature.HasDeployments {
		return g.Group([]g.Node{
			h.Div(h.Class("table-actions"), toggleLink),
			envTable(data.Environments, featureName),
		})
	}

	groups, ungrouped := groupByDeployment(data.Environments)
	return g.Group([]g.Node{
		h.Div(h.Class("table-actions"), toggleLink),
		deploymentStatusTable(groups, ungrouped, featureName),
	})
}

func groupByDeployment(envs []EnvironmentStatus) ([]deploymentGroup, []EnvironmentStatus) {
	groups := map[string]*deploymentGroup{}
	var order []string
	var ungrouped []EnvironmentStatus

	for _, env := range envs {
		if !env.HasDeployments {
			ungrouped = append(ungrouped, env)
			continue
		}
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
		result = append(result, *groups[id])
	}
	return result, ungrouped
}

func deploymentStatusTable(groups []deploymentGroup, ungrouped []EnvironmentStatus, featureName string) g.Node {
	var bodies []g.Node
	for _, group := range groups {
		rows := []g.Node{
			h.Tr(h.Class("deployment-group-row"),
				h.Td(g.Attr("colspan", "5"),
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
	if len(ungrouped) > 0 {
		rows := []g.Node{
			h.Tr(h.Class("deployment-group-row"),
				h.Td(g.Attr("colspan", "5"), g.Text("Not targeted by any deployment")),
			),
		}
		for _, env := range ungrouped {
			rows = append(rows, envRow(env, featureName))
		}
		bodies = append(bodies, h.TBody(g.Group(rows)))
	}
	return h.Table(h.Class("table"),
		h.THead(h.Tr(
			h.Th(g.Text("Environment")),
			h.Th(g.Text("Tenant")),
			h.Th(g.Text("Version")),
			h.Th(g.Text("Status")),
			h.Th(g.Text("Last Modified")),
		)),
		g.Group(bodies),
	)
}

func envRow(env EnvironmentStatus, featureName string) g.Node {
	return h.Tr(
		h.Td(h.A(h.Href("/tenants/"+env.TenantSlug+"/envs/"+env.Name+"/"+featureName), g.Text(env.Name))),
		h.Td(g.Text(env.TenantName)),
		h.Td(versionCell(env)),
		h.Td(rolloutStatus(env.StatusText)),
		h.Td(g.Text(env.LastModified)),
	)
}

func versionCell(env EnvironmentStatus) g.Node {
	if env.ReleaseVersion == "" {
		return g.Text("")
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

func envTable(envs []EnvironmentStatus, featureName string) g.Node {
	return h.Table(h.Class("table sortable"),
		h.THead(h.Tr(
			h.Th(g.Text("Environment")),
			h.Th(g.Text("Tenant")),
			h.Th(g.Text("Status")),
			h.Th(g.Text("Last Modified")),
		)),
		h.TBody(g.Group(g.Map(envs, func(env EnvironmentStatus) g.Node {
			return h.Tr(
				h.Td(h.A(h.Href("/tenants/"+env.TenantSlug+"/envs/"+env.Name+"/"+featureName), g.Text(env.Name))),
				h.Td(g.Text(env.TenantName)),
				h.Td(rolloutStatus(env.StatusText)),
				h.Td(g.Text(env.LastModified)),
			)
		}))),
	)
}

func deploymentsTab(data *DetailPage) g.Node {
	if len(data.Deployments) == 0 {
		return h.P(g.Text("No deployments yet."))
	}
	return h.Table(h.Class("table sortable"), h.THead(h.Tr(h.Th(g.Text("ID")), h.Th(g.Text("Version")), h.Th(g.Text("Status")), h.Th(g.Text("Target")), h.Th(g.Text("Created")))), h.TBody(g.Group(g.Map(data.Deployments, func(dep DeploymentItem) g.Node {
		return h.Tr(h.Td(h.A(h.Href("/deployments/"+dep.ID), g.Text(dep.ID[:8]))), h.Td(g.Text(dep.Version)), h.Td(g.Text(dep.Status)), h.Td(g.Text(dep.Target)), h.Td(g.Text(dep.Created)))
	}))))
}

func rolloutsTab(data *DetailPage) g.Node {
	if len(data.Rollouts) == 0 {
		return h.P(g.Text("No rollouts yet."))
	}
	return h.Table(h.Class("table sortable"), h.THead(h.Tr(h.Th(g.Text("Version")), h.Th(g.Text("Status")), h.Th(g.Text("Target")), h.Th(g.Text("Created")), h.Th(g.Text("Completed")))), h.TBody(g.Group(g.Map(data.Rollouts, func(rollout RolloutItem) g.Node {
		return h.Tr(h.Td(rolloutVersionCell(rollout)), h.Td(rolloutStatus(rollout.Status)), h.Td(g.Text(rollout.Target)), h.Td(g.Text(rollout.Created)), h.Td(g.Text(completedDate(rollout.Completed))))
	}))))
}

func rolloutVersionCell(r RolloutItem) g.Node {
	return h.A(h.Href("/rollouts/"+r.FeatureName+"/"+r.Version), g.Text(r.Version))
}

func dependencyLinks(feature *Feature) g.Node {
	names := []string{}
	for _, dependency := range feature.Dependencies {
		names = append(names, dependency.AllOf...)
		names = append(names, dependency.AnyOf...)
	}
	if len(names) == 0 {
		return g.Text(" -")
	}
	links := make([]g.Node, 0, len(names)*2)
	for _, name := range names {
		links = append(links, g.Text(" "), h.A(h.Href("/features/"+name), g.Text(name)))
	}
	return g.Group(links)
}

func rolloutStatus(status string) g.Node {
	switch strings.ToUpper(status) {
	case "DEPLOYED":
		return g.Group([]g.Node{h.Span(h.Class("status-success"), g.Text("✓")), g.Text(" DEPLOYED")})
	case "FAILED":
		return g.Group([]g.Node{h.Span(h.Class("status-error"), g.Text("✗")), g.Text(" FAILED")})
	case "PENDING", "PENDING-INSTALL", "PENDING-UPGRADE", "PENDING-ROLLBACK":
		return g.Group([]g.Node{h.Span(h.Class("status-pending"), g.Text("⏳")), g.Text(" PENDING")})
	case "DISABLED":
		return g.Group([]g.Node{h.Span(h.Class("status-disabled"), g.Text("○")), g.Text(" DISABLED")})
	case "CREATED":
		return g.Group([]g.Node{h.Span(h.Class("status-pending"), g.Text("⏳")), g.Text(" CREATED")})
	case "UNKNOWN":
		return g.Group([]g.Node{h.Span(h.Class("status-pending"), g.Text("?")), g.Text(" UNKNOWN")})
	case "ENABLED":
		return g.Group([]g.Node{h.Span(h.Class("status-success"), g.Text("✓")), g.Text(" ENABLED")})
	default:
		return g.Text(status)
	}
}

func completedDate(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func loadFeatureData(r *http.Request, repo database.Repo, activeTab string) (*DetailPage, error) {
	featureName := chi.URLParam(r, "feature")
	features, err := featurepkg.Features(r.Context())
	if err != nil {
		return nil, err
	}
	feature, err := featurepkg.FeatureByName(r.Context(), featureName)
	if err != nil {
		return nil, err
	}
	data := &DetailPage{Breadcrumbs: []breadcrumb.Crumb{breadcrumb.Features(), breadcrumb.Feature(featureName)}, Features: toFeatureNavs(features), CurrentFeature: &Feature{Feature: feature, Config: featureConfigItems(feature)}, ActiveTab: activeTab}
	if activeTab == "status" {
		showParam := r.URL.Query().Get("show")
		data.ShowAll = showParam == "all"
		data.Environments = featureEnvironmentStatuses(r.Context(), repo, feature, data.ShowAll)
	}
	if activeTab == "deployments" {
		data.Deployments = featureDeployments(r.Context(), featureName)
	}
	if activeTab == "rollouts" {
		data.Rollouts = featureRollouts(r.Context(), repo, featureName)
	}
	return data, nil
}

func featureConfigItems(feature *model.Feature) []ConfigItem {
	items := make([]ConfigItem, 0, len(feature.Values))
	keys := make([]string, 0, len(feature.Values))
	for key, value := range feature.Values {
		if value.Config != nil {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := feature.Values[key]
		item := ConfigItem{Key: key, Value: rawValueToString(feature.ValuesYAML[key])}
		item.Type = strings.ToUpper(value.Config.Type.String())
		item.IsSecret = value.Config.Secret
		if value.Computed != nil {
			item.IsComputed = true
			item.Template = value.Computed.Template
		}
		items = append(items, item)
	}
	return items
}

type envStatusEntry struct {
	state        string
	deploymentID string
	targetLabels map[string]string
	version      string
	created      time.Time
}

type deploymentGroup struct {
	DeploymentID string
	Labels       map[string]string
	Environments []EnvironmentStatus
}

func featureEnvironmentStatuses(ctx context.Context, repo database.Repo, feature *model.Feature, showAll bool) []EnvironmentStatus {
	ret := []EnvironmentStatus{}
	deploymentStatuses := map[uuid.UUID]envStatusEntry{}
	if feature.HasDeployments {
		deployments, err := deployment.ListDeploymentsByFeature(ctx, feature.Name)
		if err == nil {
			for _, dep := range deployments {
				statuses, err := deployment.ListDeploymentStatuses(ctx, dep.ID)
				if err != nil {
					continue
				}
				for _, status := range statuses {
					existing, exists := deploymentStatuses[status.EnvironmentID]
					if exists && !deployment.IsMoreSpecific(dep.TargetLabels, existing.targetLabels, dep.Created, existing.created) {
						continue
					}
					deploymentStatuses[status.EnvironmentID] = envStatusEntry{
						state:        string(status.State),
						deploymentID: dep.ID.String(),
						targetLabels: dep.TargetLabels,
						version:      dep.Feature.Version,
						created:      dep.Created,
					}
				}
			}
		}
	}
	tenants, err := envpkg.GetTenants(ctx)
	if err != nil {
		return ret
	}
	for _, tenant := range tenants {
		envs, err := repo.EnvironmentsGet(ctx, tenant.ID)
		if err != nil {
			continue
		}
		for _, env := range envs {
			if !featureTargetsKind(feature.EnvironmentKinds, env.Kind) {
				continue
			}
			state, err := featurepkg.FeatureStateGet(ctx, env.ID, feature.Name)
			if err != nil {
				continue
			}

			environmentStatus := EnvironmentStatus{
				Name:              env.Name,
				TenantName:        tenant.Name,
				TenantSlug:        tenant.Name,
				Enabled:           state.Enabled,
				LastModified:      formatTime(state.LastModified),
				FeatureDeployable: feature.HasDeployments,
			}

			releases, err := repo.ReleaseStatusesGet(ctx, env.ID)
			if err == nil {
				for _, release := range releases {
					if release.Name == feature.Name {
						environmentStatus.ReleaseVersion = release.Version
						break
					}
				}
			}

			if feature.HasDeployments {
				if deploymentStatus, ok := deploymentStatuses[env.ID]; ok {
					environmentStatus.HasDeployments = true
					environmentStatus.StatusText = deploymentStatus.state
					environmentStatus.DeploymentID = deploymentStatus.deploymentID
					environmentStatus.DeploymentVersion = deploymentStatus.version
					environmentStatus.TargetLabels = deploymentStatus.targetLabels
				}
			}
			if !environmentStatus.HasDeployments {
				if environmentStatus.FeatureDeployable {
					environmentStatus.StatusText = "-"
				} else {
					for _, release := range releases {
						if release.Name == feature.Name {
							environmentStatus.StatusText = release.Status
							break
						}
					}
					if environmentStatus.StatusText == "" {
						if state.Enabled {
							environmentStatus.StatusText = "Enabled"
						} else {
							environmentStatus.StatusText = "Disabled"
						}
					}
				}
			}

			ret = append(ret, environmentStatus)
		}
	}
	if showAll {
		return ret
	}
	filtered := make([]EnvironmentStatus, 0, len(ret))
	for _, environmentStatus := range ret {
		if environmentStatus.Enabled || (environmentStatus.HasDeployments && environmentStatus.DeploymentID != "") {
			filtered = append(filtered, environmentStatus)
		}
	}
	return filtered
}

func featureDeployments(ctx context.Context, featureName string) []DeploymentItem {
	deployments, err := deployment.ListDeploymentsByFeature(ctx, featureName)
	if err != nil {
		return nil
	}
	items := make([]DeploymentItem, 0, len(deployments))
	for _, dep := range deployments {
		statuses, _ := deployment.ListDeploymentStatuses(ctx, dep.ID)
		state, _ := aggregateDeploymentStatus(statuses)
		target := deploymentTarget(dep)
		items = append(items, DeploymentItem{
			ID:      dep.ID.String(),
			Version: dep.Feature.Version,
			Status:  state,
			Target:  target,
			Created: formatTime(dep.Created),
		})
	}
	return items
}

func aggregateDeploymentStatus(statuses []*deployment.DeploymentStatus) (string, int) {
	if len(statuses) == 0 {
		return "UNKNOWN", 0
	}

	disabledCount := 0
	for _, s := range statuses {
		if s.State == deployment.DeploymentStatusStateDisabled {
			disabledCount++
		}
	}

	if disabledCount == len(statuses) {
		return "DISABLED", disabledCount
	}

	allDeployed := true
	for _, s := range statuses {
		if s.State == deployment.DeploymentStatusStateDisabled {
			continue
		}
		switch s.State {
		case deployment.DeploymentStatusStateFailed:
			return "FAILED", disabledCount
		case deployment.DeploymentStatusStatePending, deployment.DeploymentStatusStateCreated:
			allDeployed = false
		case deployment.DeploymentStatusStateDeployed:
		default:
			allDeployed = false
		}
	}

	if allDeployed {
		return "DEPLOYED", disabledCount
	}

	return "PENDING", disabledCount
}

func deploymentTarget(dep *deployment.Deployment) string {
	labels := dep.Target()
	if len(labels) == 0 {
		return "All environments"
	}

	keys := make([]string, 0, len(labels))
	labelMap := make(map[string]string, len(labels))
	for _, label := range labels {
		keys = append(keys, label.Key)
		labelMap[label.Key] = label.Value
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+labelMap[k])
	}
	return strings.Join(parts, ", ")
}

func featureRollouts(ctx context.Context, repo database.Repo, featureName string) []RolloutItem {
	rollouts, err := repo.RolloutsForFeature(ctx, featureName)
	if err != nil {
		return nil
	}
	items := make([]RolloutItem, 0, len(rollouts))
	for _, rollout := range rollouts {
		items = append(items, RolloutItem{FeatureName: rollout.FeatureName, Version: rollout.Version, Status: strings.ToUpper(rollout.Status.String()), Created: formatTime(rollout.Created), Completed: formatTimePtr(rollout.Completed)})
	}
	return items
}

func featureTargetsKind(kinds []model.EnvironmentKind, envKind model.EnvironmentKind) bool {
	if len(kinds) == 0 {
		return true
	}
	return slices.Contains(kinds, envKind)
}

func configTypeLabel(item ConfigItem) string {
	if item.IsComputed {
		return "computed"
	}
	if item.IsSecret {
		return "secret"
	}
	if item.Type != "" {
		return strings.ToLower(item.Type)
	}
	return "-"
}

func configDefaultValue(item ConfigItem) g.Node {
	if item.IsSecret {
		return h.Span(h.Class("text-muted"), g.Text("••••••••"))
	}
	if item.IsComputed {
		return h.Code(g.Text(item.Template))
	}
	if item.Value != "" {
		return h.Span(g.Text(item.Value))
	}
	return h.Span(h.Class("text-muted"), g.Text("-"))
}

func toFeatureNavs(features []*model.Feature) []view.FeatureNav {
	ret := make([]view.FeatureNav, 0, len(features))
	for _, feature := range features {
		ret = append(ret, view.FeatureNav{Name: feature.Name})
	}
	return ret
}

func rawValueToString(value json.RawMessage) string {
	if len(value) == 0 {
		return ""
	}

	var v any
	if err := json.Unmarshal(value, &v); err != nil {
		return string(value)
	}
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return string(value)
	}
	return string(b)
}

var oslo = mustLoadLocation("Europe/Oslo")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(oslo).Format("2006-01-02 15:04:05")
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return formatTime(*t)
}
