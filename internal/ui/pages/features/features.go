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
	"github.com/nais/fasit/internal/database"
	envpkg "github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/ui"
	"github.com/nais/fasit/internal/ui/breadcrumb"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, layout.Props)

type DetailPage struct {
	Breadcrumbs    []breadcrumb.Crumb
	Features       []view.FeatureNav
	CurrentFeature *Feature
	Environments   []EnvironmentStatus
	Rollouts       []RolloutItem
	ActiveTab      string
}

type EnvironmentStatus struct {
	Name         string
	TenantName   string
	TenantSlug   string
	Enabled      bool
	LastModified string
	Created      string
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
	FeatureName  string
	Version      string
	Status       string
	Created      string
	Completed    string
	Target       string
	DeploymentID string
}

func ListHandler(renderPage RenderPage, _ database.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		features, err := featurepkg.Features(r.Context())
		if err != nil {
			http.Error(w, "Failed to load features", http.StatusInternalServerError)
			return
		}
		renderPage(w, layout.Props{Title: "Features", CurrentSection: "features", Content: listPage(toFeatureNavs(features))})
	}
}

func TabHandler(renderPage RenderPage, repo database.Repo, activeTab string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := loadFeatureData(r, repo, activeTab)
		if err != nil {
			http.Error(w, "Failed to load feature data", http.StatusInternalServerError)
			return
		}
		renderPage(w, layout.Props{Title: data.CurrentFeature.Name, CurrentSection: "features", Content: detailPage(data)})
	}
}

func listPage(features []view.FeatureNav) g.Node {
	return h.Div(h.Class("container"), components.FeaturesSidebar(features, ""), h.Main(h.Class("main-content"), components.Breadcrumbs([]breadcrumb.Crumb{breadcrumb.Features()})))
}

func detailPage(data *DetailPage) g.Node {
	return h.Div(h.Class("container"), components.FeaturesSidebar(data.Features, data.CurrentFeature.Name), h.Main(h.Class("main-content"), components.Breadcrumbs(data.Breadcrumbs), h.Div(h.Class("card"), h.Div(h.Class("card-body"), components.TabsNav(data.ActiveTab, featureTabs()), h.Div(h.Class("tab-content-wrapper"), tabContent(data))))))
}

func featureTabs() []components.Tab {
	return []components.Tab{{ID: "overview", Href: "./", Label: "Overview"}, {ID: "status", Href: "./status", Label: "Status"}, {ID: "rollouts", Href: "./rollouts", Label: "Rollouts"}}
}

func tabContent(data *DetailPage) g.Node {
	switch data.ActiveTab {
	case "status":
		return statusTab(data)
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
	if len(data.Environments) == 0 {
		return h.P(g.Text("No environments found."))
	}
	return h.Table(h.Class("table sortable"), h.THead(h.Tr(h.Th(g.Text("Environment")), h.Th(g.Text("Tenant")), h.Th(g.Text("Status")), h.Th(g.Text("Created")), h.Th(g.Text("Last Modified")))), h.TBody(g.Group(g.Map(data.Environments, func(environment EnvironmentStatus) g.Node {
		statusClass, statusIcon, statusText := "status-disabled", "○", "Disabled"
		if environment.Enabled {
			statusClass, statusIcon, statusText = "status-success", "✓", "Enabled"
		}
		return h.Tr(h.Td(h.A(h.Href(ui.BasePath+"/tenants/"+environment.TenantSlug+"/envs/"+environment.Name+"/"+data.CurrentFeature.Name+"/"), g.Text(environment.Name))), h.Td(h.A(h.Href(ui.BasePath+"/tenants/"+environment.TenantSlug), g.Text(environment.TenantName))), h.Td(h.Span(h.Class(statusClass), g.Text(statusIcon)), g.Text(" "+statusText)), h.Td(g.Text(environment.Created)), h.Td(g.Text(environment.LastModified)))
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
	if r.DeploymentID != "" {
		return h.A(h.Href(ui.BasePath+"/deployments/"+r.DeploymentID+"/"), g.Text(r.Version))
	}
	return h.A(h.Href(ui.BasePath+"/rollouts/"+r.FeatureName+"/"+r.Version+"/"), g.Text(r.Version))
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
		links = append(links, g.Text(" "), h.A(h.Href(ui.BasePath+"/features/"+name), g.Text(name)))
	}
	return g.Group(links)
}

func rolloutStatus(status string) g.Node {
	switch strings.ToUpper(status) {
	case "DEPLOYED":
		return g.Group([]g.Node{h.Span(h.Class("status-success"), g.Text("✓")), g.Text(" DEPLOYED")})
	case "FAILED":
		return g.Group([]g.Node{h.Span(h.Class("status-error"), g.Text("✗")), g.Text(" FAILED")})
	case "PENDING":
		return g.Group([]g.Node{h.Span(h.Class("status-pending"), g.Text("⏳")), g.Text(" PENDING")})
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
		data.Environments = featureEnvironmentStatuses(r.Context(), repo, feature)
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

func featureEnvironmentStatuses(ctx context.Context, repo database.Repo, feature *model.Feature) []EnvironmentStatus {
	ret := []EnvironmentStatus{}
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
			ret = append(ret, EnvironmentStatus{Name: env.Name, TenantName: tenant.Name, TenantSlug: tenant.Name, Enabled: state.Enabled, Created: formatTimePtr(state.EnabledAt), LastModified: formatTime(state.LastModified)})
		}
	}
	return ret
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
		return h.Span(h.Class("text-muted"), g.Text("[SECRET]"))
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
