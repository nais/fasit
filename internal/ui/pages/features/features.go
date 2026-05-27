package features

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/deployment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/ui/breadcrumb"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/nais/fasit/internal/ui/pages/auditlog"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, *http.Request, layout.Props)

type DetailPage struct {
	Breadcrumbs    []breadcrumb.Crumb
	Features       []view.FeatureNav
	CurrentFeature *model.Feature
	DeploymentEnvs []DeploymentEnvStatus
	RecentActivity []*audit.Entry
	ActiveTab      string
	ConfigItems    []components.ConfigItem
	ExplorerData   *configExplorerData
}

func ListHandler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		features, err := featurepkg.FeatureNames(r.Context())
		if err != nil {
			http.Error(w, "Failed to load features", http.StatusInternalServerError)
			return
		}

		deps, _ := deployment.List(r.Context())
		audits, _ := audit.ListRecent(r.Context(), 200)
		deploymentActors := deploymentActorsByID(audits)

		var depRows []depRow
		for _, dep := range deps {
			statuses, err := deployment.ListDeploymentStatuses(r.Context(), dep.ID)
			if err != nil || len(statuses) == 0 {
				depRows = append(depRows, depRow{
					FeatureName: dep.Feature.Name,
					Version:     dep.Feature.Version,
					Status:      "UNKNOWN",
					Created:     dep.Created,
					DepID:       dep.ID.String(),
					Actor:       deploymentActors[dep.ID.String()],
				})
			} else {
				for _, s := range statuses {
					depRows = append(depRows, depRow{
						FeatureName: dep.Feature.Name,
						Version:     dep.Feature.Version,
						Status:      string(s.State),
						Created:     dep.Created,
						DepID:       dep.ID.String(),
						Actor:       deploymentActors[dep.ID.String()],
					})
				}
			}
		}

		sort.Slice(depRows, func(i, j int) bool {
			return depRows[i].Created.After(depRows[j].Created)
		})

		renderPage(w, r, layout.Props{Title: "Home", CurrentPage: components.PageHome, Content: listPage(toFeatureNavs(features), depRows, audits), HideHeaderSearch: true})
	}
}

func IndexHandler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		indexRows, err := featurepkg.FeatureIndexRows(r.Context())
		if err != nil {
			http.Error(w, "Failed to load features", http.StatusInternalServerError)
			return
		}

		query := strings.TrimSpace(r.URL.Query().Get("q"))
		rows := make([]featureIndexRow, 0, len(indexRows))
		for _, indexRow := range indexRows {
			row := featureIndexRow{
				Name:        indexRow.Name,
				Description: indexRow.Description,
				Source:      indexRow.Source,
			}
			if query == "" || featureIndexMatches(row, query) {
				rows = append(rows, row)
			}
		}
		sort.Slice(rows, func(i, j int) bool {
			return rows[i].Name < rows[j].Name
		})

		renderPage(w, r, layout.Props{Title: "Features", CurrentPage: components.PageFeatures, Content: featureIndexPage(rows, query)})
	}
}

func Handler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := loadFeatureData(r)
		if err != nil {
			http.Error(w, "Failed to load feature data", http.StatusInternalServerError)
			return
		}
		data.ActiveTab = "overview"
		data.RecentActivity, _ = audit.ListForFeature(r.Context(), data.CurrentFeature.Name, 10)
		setFeatureBreadcrumbSubtitle(data)
		renderPage(w, r, layout.Props{Title: data.CurrentFeature.Name, CurrentPage: components.PageFeatures, Content: detailPage(data)})
	}
}

func DeploySpecsHandler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := loadFeatureData(r)
		if err != nil {
			http.Error(w, "Failed to load feature data", http.StatusInternalServerError)
			return
		}
		data.ActiveTab = "deploy-specs"
		renderPage(w, r, layout.Props{Title: data.CurrentFeature.Name + " · Deploy specs", CurrentPage: components.PageFeatures, Content: detailPage(data)})
	}
}

func ConfigTabHandler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := loadFeatureData(r)
		if err != nil {
			http.Error(w, "Failed to load feature data", http.StatusInternalServerError)
			return
		}
		data.ActiveTab = "config"
		data.ConfigItems, err = loadGlobalConfigItems(r.Context(), data.CurrentFeature)
		if err != nil {
			http.Error(w, "Failed to load config", http.StatusInternalServerError)
			return
		}
		renderPage(w, r, layout.Props{Title: data.CurrentFeature.Name + " · Config", CurrentPage: components.PageFeatures, Content: detailPage(data)})
	}
}

func ConfigExplorerHandler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := loadFeatureData(r)
		if err != nil {
			http.Error(w, "Failed to load feature data", http.StatusInternalServerError)
			return
		}
		data.ActiveTab = "config-explorer"
		data.ExplorerData, err = loadConfigExplorerData(r.Context(), data.CurrentFeature)
		if err != nil {
			http.Error(w, "Failed to load config explorer", http.StatusInternalServerError)
			return
		}
		data.ExplorerData.SelectedKeys = parseExplorerKeys(r, data.ExplorerData.AllKeys)
		renderComputedCells(r.Context(), data.CurrentFeature, data.ExplorerData, data.ExplorerData.SelectedKeys)
		renderPage(w, r, layout.Props{Title: data.CurrentFeature.Name + " · Config Explorer", CurrentPage: components.PageFeatures, Content: detailPage(data)})
	}
}

func listPage(features []view.FeatureNav, deps []depRow, audits []*audit.Entry) g.Node {
	return h.Div(h.Class("container"),
		h.Main(h.Class("main-content landing-page"),
			landingSearch(features),
			components.CardCompact(
				recentDeployments(deps),
			),
			components.CardCompact(
				recentActivity(audits),
			),
		),
	)
}

func landingSearch(features []view.FeatureNav) g.Node {
	return h.Section(h.Class("landing-search"),
		h.Form(h.Method("get"), h.Action("/search"), h.Class("feature-search-form landing-search-form"), g.Attr("data-feature-search", ""),
			h.Input(
				h.Type("search"),
				h.Name("q"),
				h.Class("feature-search-input landing-search-input"),
				h.Placeholder("Search features… (Ctrl+K)"),
				h.AutoComplete("off"),
				h.AutoFocus(),
				g.Attr("aria-label", "Search features"),
			),
			h.Div(h.Class("feature-search-suggestions"), g.Attr("data-feature-search-suggestions", "")),
		),
	)
}

type featureIndexRow struct {
	Name        string
	Description string
	Source      string
}

func featureIndexPage(features []featureIndexRow, query string) g.Node {
	return h.Div(h.Class("container"),
		h.Main(h.Class("main-content landing-page"),
			components.CardCompact(
				h.Div(h.Class("feature-index-header"),
					h.H1(g.Text("Features")),
				),
				h.Div(h.Class("feature-index-toolbar"),
					h.Input(h.Type("search"), h.Name("q"), h.Value(query), h.Class("feature-index-filter"), h.Placeholder("Filter features…"), g.Attr("data-url-filter", "q"), g.Attr("aria-label", "Filter features")),
				),
				featureIndexTable(features),
			),
		),
	)
}

func featureIndexMatches(feature featureIndexRow, query string) bool {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return true
	}
	text := strings.ToLower(strings.Join([]string{
		feature.Name,
		feature.Description,
		feature.Source,
	}, " "))
	for _, term := range terms {
		if !strings.Contains(text, term) {
			return false
		}
	}
	return true
}

func featureIndexTable(features []featureIndexRow) g.Node {
	rows := make([]g.Node, 0, len(features))
	for _, feature := range features {
		rows = append(rows, h.Tr(
			h.Td(h.A(h.Href("/features/"+feature.Name), g.Text(feature.Name))),
			h.Td(h.Class("text-muted"), g.Text(feature.Description)),
			h.Td(g.If(feature.Source != "", h.A(h.Href(feature.Source), h.Target("_blank"), h.Rel("noopener noreferrer"), g.Text("GitHub ↗")))),
		))
	}
	return h.Table(h.ID("feature-index-table"), h.Class("table table-compact sortable"),
		h.THead(h.Tr(
			h.Th(g.Text("Feature")),
			h.Th(g.Text("Description")),
			h.Th(g.Text("Source")),
		)),
		h.TBody(g.Group(rows)),
	)
}

func featureIndexStatusKey(env DeploymentEnvStatus) string {
	if !env.Enabled || env.EnvReconcileDisabled || strings.EqualFold(env.StatusText, "DISABLED") {
		return "disabled"
	}
	switch strings.ToUpper(env.StatusText) {
	case "DEPLOYED":
		return "ok"
	case "FAILED":
		return "failed"
	case "PENDING", "CREATED":
		return "pending"
	default:
		return "unknown"
	}
}

type depRow struct {
	FeatureName string
	Version     string
	Status      string
	Created     time.Time
	DepID       string
	Actor       string
}

func deploymentActorsByID(audits []*audit.Entry) map[string]string {
	ret := make(map[string]string)
	for _, a := range audits {
		if a.ObjectType != audit.ObjectTypeDeployment || a.Action != audit.ActionCreated || len(a.Metadata) == 0 {
			continue
		}
		var metadata struct {
			DeploymentID string `json:"deploymentId"`
		}
		if err := json.Unmarshal(a.Metadata, &metadata); err != nil || metadata.DeploymentID == "" {
			continue
		}
		ret[metadata.DeploymentID] = a.Actor
	}
	return ret
}

func recentDeployments(rows []depRow) g.Node {
	if len(rows) == 0 {
		return h.P(h.Class("text-muted"), g.Text("No deployments."))
	}

	// Aggregate by feature+version
	type aggKey struct{ feature, version string }
	type aggRow struct {
		FeatureName string
		Version     string
		Statuses    []string
		Latest      time.Time
		Actor       string
	}
	seen := make(map[aggKey]*aggRow)
	var ordered []aggKey
	for _, r := range rows {
		k := aggKey{r.FeatureName, r.Version}
		if agg, ok := seen[k]; ok {
			agg.Statuses = append(agg.Statuses, r.Status)
			if r.Created.After(agg.Latest) {
				agg.Latest = r.Created
				agg.Actor = r.Actor
			}
		} else {
			seen[k] = &aggRow{
				FeatureName: r.FeatureName,
				Version:     r.Version,
				Statuses:    []string{r.Status},
				Latest:      r.Created,
				Actor:       r.Actor,
			}
			ordered = append(ordered, k)
		}
	}

	if len(ordered) > 10 {
		ordered = ordered[:10]
	}
	tableRows := make([]g.Node, 0, len(ordered))
	for _, k := range ordered {
		agg := seen[k]
		tableRows = append(tableRows, h.Tr(
			h.Td(h.A(h.Href("/features/"+agg.FeatureName), g.Text(agg.FeatureName))),
			h.Td(h.Class("text-muted"), g.Text(agg.Version)),
			h.Td(renderAggStatus(agg.Statuses)),
			deploymentActorCell(agg.Actor),
			h.Td(h.Class("text-muted text-right"), h.Title(view.FormatTime(agg.Latest)), g.Text(view.RelativeTime(agg.Latest))),
		))
	}

	return g.Group([]g.Node{
		h.H3(g.Text("Recent deployments")),
		h.Table(h.Class("table table-compact"),
			h.THead(h.Tr(
				h.Th(g.Text("Feature")),
				h.Th(g.Text("Version")),
				h.Th(g.Text("Status")),
				h.Th(g.Text("Actor")),
				h.Th(h.Class("text-right"), g.Text("When")),
			)),
			h.TBody(g.Group(tableRows)),
		),
		h.Div(h.Style("margin-top: 0.75rem; font-size: 0.85rem;"), h.A(h.Href("/deployments"), h.Class("link-muted"), g.Text("All deployments →"))),
	})
}

func deploymentActorCell(actor string) g.Node {
	if actor == "" {
		return h.Td(h.Class("text-muted"), g.Text("—"))
	}
	return h.Td(view.ActorNode(actor))
}

func recentActivity(audits []*audit.Entry) g.Node {
	filtered := make([]*audit.Entry, 0, len(audits))
	for _, a := range audits {
		if a.ObjectType == audit.ObjectTypeDeployment && a.Action == audit.ActionCreated {
			continue
		}
		filtered = append(filtered, a)
		if len(filtered) == 10 {
			break
		}
	}
	if len(filtered) == 0 {
		return h.P(h.Class("text-muted"), g.Text("No recent activity."))
	}

	tableRows := make([]g.Node, 0, len(filtered))
	for _, a := range filtered {
		resource := resourceLinkNode(a)
		tableRows = append(tableRows, h.Tr(
			h.Td(g.Text(string(a.Action))),
			h.Td(resource),
			h.Td(h.Class("text-muted"), g.Text(a.Description)),
			h.Td(view.ActorNode(a.Actor)),
			h.Td(h.Class("text-muted"), h.Title(view.FormatTime(a.CreatedAt)), g.Text(view.RelativeTime(a.CreatedAt))),
		))
	}

	return g.Group([]g.Node{
		h.H3(g.Text("Recent activity")),
		h.Table(h.Class("table table-compact"),
			h.THead(h.Tr(
				h.Th(g.Text("Action")),
				h.Th(g.Text("Resource")),
				h.Th(g.Text("Details")),
				h.Th(g.Text("Actor")),
				h.Th(g.Text("When")),
			)),
			h.TBody(g.Group(tableRows)),
		),
		h.Div(h.Style("margin-top: 0.75rem; font-size: 0.85rem;"), h.A(h.Href("/auditlog"), h.Class("link-muted"), g.Text("All activity →"))),
	})
}

type aggStatus struct {
	class string // "status-success", "status-pending", "status-error"
	label string
}

func computeAggStatus(statuses []string) aggStatus {
	var failed, deployed, pending, total int
	for _, s := range statuses {
		total++
		switch strings.ToUpper(s) {
		case "FAILED":
			failed++
		case "DEPLOYED", "DISABLED":
			deployed++
		case "PENDING", "PENDING-INSTALL", "PENDING-UPGRADE", "PENDING-ROLLBACK", "CREATED":
			pending++
		}
	}
	if deployed == total {
		return aggStatus{"status-success", "Deployed"}
	}
	if failed == 0 {
		return aggStatus{"status-pending", fmt.Sprintf("%d/%d deployed", deployed, total)}
	}
	var parts []string
	if deployed > 0 {
		parts = append(parts, fmt.Sprintf("%d deployed", deployed))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}
	if pending > 0 {
		parts = append(parts, fmt.Sprintf("%d pending", pending))
	}
	other := total - deployed - failed - pending
	if other > 0 {
		parts = append(parts, fmt.Sprintf("%d unknown", other))
	}
	return aggStatus{"status-error", strings.Join(parts, ", ")}
}

func renderAggStatus(statuses []string) g.Node {
	s := computeAggStatus(statuses)
	var icon string
	switch s.class {
	case "status-success":
		icon = "\u2713"
	case "status-pending":
		icon = "\u23f3"
	case "status-error":
		icon = "\u2717"
	}
	return g.Group([]g.Node{h.Span(h.Class(s.class), g.Text(icon)), g.Text(" " + s.label)})
}

func detailPage(data *DetailPage) g.Node {
	var content g.Node
	switch data.ActiveTab {
	case "deploy-specs":
		content = deploymentSpecsContent(data)
	case "config":
		content = globalConfigContent(data)
	case "config-explorer":
		content = configExplorerContent(data.CurrentFeature.Name, data.ExplorerData)
	default:
		content = featureOverviewContent(data)
	}
	return h.Div(h.Class("container"),
		featureWorkspaceSidebar(data),
		h.Main(h.Class("main-content"),
			components.Breadcrumbs(data.Breadcrumbs),
			components.Card(content),
		),
	)
}

func featureWorkspaceSidebar(data *DetailPage) g.Node {
	featureName := data.CurrentFeature.Name
	return h.Aside(h.Class("sidebar feature-workspace-sidebar"),
		h.Div(h.Class("feature-workspace-header"),
			h.H4(g.Text(featureName)),
		),
		h.Div(h.Class("nav"),
			h.Ul(
				workspaceNavItem("/features/"+featureName, "Overview", data.ActiveTab == "overview"),
				workspaceNavItem("/features/"+featureName+"/deploy-specs", "Deploy specs", data.ActiveTab == "deploy-specs"),
				workspaceNavItem("/features/"+featureName+"/config", "Config", data.ActiveTab == "config"),
				workspaceNavItem("/features/"+featureName+"/config-explorer", "Config explorer", data.ActiveTab == "config-explorer"),
			),
			h.Div(h.Class("sidebar-section-label"), g.Text("Environments")),
			h.Ul(g.Group(g.Map(featureWorkspaceEnvironments(currentDeploymentEnvStatuses(data.DeploymentEnvs)), func(env DeploymentEnvStatus) g.Node {
				return workspaceEnvironmentItem(featureName, env)
			}))),
		),
	)
}

func workspaceNavItem(href, label string, active bool) g.Node {
	attrs := []g.Node{h.Href(href)}
	if active {
		attrs = append(attrs, h.Class("active"))
	}
	return h.Li(h.A(append(attrs, g.Text(label))...))
}

func workspaceEnvironmentItem(featureName string, env DeploymentEnvStatus) g.Node {
	return h.Li(h.A(h.Href("/features/"+featureName+"/envs/"+env.TenantSlug+"/"+env.Name),
		h.Class("workspace-env-link"),
		h.Span(h.Class("workspace-env-dot "+workspaceEnvironmentStatusClass(env)), h.Title(env.StatusText)),
		h.Span(g.Text(env.TenantName+" / "+env.Name)),
	))
}

func featureWorkspaceEnvironments(envs []DeploymentEnvStatus) []DeploymentEnvStatus {
	ret := make([]DeploymentEnvStatus, 0, len(envs))
	seen := map[string]struct{}{}
	for _, env := range envs {
		key := env.TenantSlug + "/" + env.Name
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		ret = append(ret, env)
	}
	sort.Slice(ret, func(i, j int) bool {
		if ret[i].TenantName == ret[j].TenantName {
			return ret[i].Name < ret[j].Name
		}
		return ret[i].TenantName < ret[j].TenantName
	})
	return ret
}

func workspaceEnvironmentStatusClass(env DeploymentEnvStatus) string {
	switch featureIndexStatusKey(env) {
	case "ok":
		return "status-success"
	case "failed":
		return "status-error"
	case "pending":
		return "status-pending"
	case "disabled":
		return "status-disabled"
	default:
		return "text-muted"
	}
}

func featureOverviewContent(data *DetailPage) g.Node {
	if len(data.RecentActivity) == 0 {
		return h.Div(h.Class("feature-overview"), deploymentDetailContent(data))
	}
	return h.Div(h.Class("feature-overview feature-overview-split"),
		h.Div(h.Class("feature-overview-deployments"), deploymentDetailContent(data)),
		h.Aside(h.Class("feature-overview-activity"), featureRecentActivityCompact(data.CurrentFeature.Name, data.RecentActivity)),
	)
}

func featureRecentActivityCompact(featureName string, audits []*audit.Entry) g.Node {
	items := make([]g.Node, 0, len(audits))
	for _, a := range audits {
		description := auditlog.Description(a)
		items = append(items, h.Li(
			h.Div(h.Class("feature-activity-meta"),
				h.Span(
					g.Text(string(a.Action)),
					g.If(a.Actor != "", g.Group([]g.Node{g.Text(" by "), h.Span(h.Class("feature-activity-actor"), view.ActorNode(a.Actor))})),
				),
				h.Span(h.Title(view.FormatTime(a.CreatedAt)), g.Text(view.RelativeTime(a.CreatedAt))),
			),
			h.Div(h.Class("feature-activity-resource"), resourceLinkNode(a)),
			g.If(description != "", h.Div(h.Class("feature-activity-description"), g.Text(description))),
		))
	}

	return g.Group([]g.Node{
		h.Div(h.Class("feature-activity-header"),
			h.H3(g.Text("Recent activity")),
			h.A(h.Href("/auditlog?q="+url.QueryEscape(featureName)), h.Class("link-muted"), g.Text("All →")),
		),
		h.Ul(h.Class("feature-activity-list"), g.Group(items)),
	})
}

func loadFeatureData(r *http.Request) (*DetailPage, error) {
	featureName := chi.URLParam(r, "feature")
	features, err := featurepkg.FeatureNames(r.Context())
	if err != nil {
		return nil, err
	}
	feature, err := featurepkg.FeatureByName(r.Context(), featureName)
	if err != nil {
		return nil, err
	}
	featureCrumb := breadcrumb.Feature(featureName)
	featureCrumb.SourceURL = feature.Source
	data := &DetailPage{
		Breadcrumbs:    []breadcrumb.Crumb{breadcrumb.Features(), featureCrumb},
		Features:       toFeatureNavs(features),
		CurrentFeature: feature,
	}
	loadDeploymentData(r.Context(), feature, data)
	return data, nil
}

func setFeatureBreadcrumbSubtitle(data *DetailPage) {
	if len(data.Breadcrumbs) == 0 || data.CurrentFeature.Description == "" {
		return
	}
	data.Breadcrumbs[len(data.Breadcrumbs)-1].Subtitle = data.CurrentFeature.Description
}

func toFeatureNavs(names []string) []view.FeatureNav {
	ret := make([]view.FeatureNav, 0, len(names))
	for _, name := range names {
		ret = append(ret, view.FeatureNav{
			Name: name,
		})
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name < ret[j].Name
	})
	return ret
}

func featureTargetsKind(kinds []model.EnvironmentKind, envKind model.EnvironmentKind) bool {
	if len(kinds) == 0 {
		return true
	}
	return slices.Contains(kinds, envKind)
}

func lastDeployedCell(t time.Time, extraTitle string) g.Node {
	if t.IsZero() {
		if extraTitle != "" {
			return h.Td(h.Title(extraTitle), h.Span(h.Class("text-muted"), g.Text("never")))
		}
		return h.Td(h.Span(h.Class("text-muted"), g.Text("never")))
	}
	title := view.FormatTime(t)
	if extraTitle != "" {
		title = extraTitle
	}
	return h.Td(h.Title(title), g.Text(view.RelativeTime(t)))
}

func renderStatus(status string) g.Node {
	switch strings.ToUpper(status) {
	case "DEPLOYED":
		return g.Group([]g.Node{h.Span(h.Class("status-success"), g.Text("✓")), g.Text(" Deployed")})
	case "FAILED":
		return g.Group([]g.Node{h.Span(h.Class("status-error"), g.Text("✗")), g.Text(" Failed")})
	case "PENDING", "PENDING-INSTALL", "PENDING-UPGRADE", "PENDING-ROLLBACK":
		return g.Group([]g.Node{h.Span(h.Class("status-pending"), g.Text("⏳")), g.Text(" Pending")})
	case "DISABLED":
		return g.Group([]g.Node{h.Span(h.Class("status-disabled"), g.Text("○")), g.Text(" Disabled")})
	case "CREATED":
		return g.Group([]g.Node{h.Span(h.Class("status-pending"), g.Text("⏳")), g.Text(" Created")})
	case "UNKNOWN":
		return g.Group([]g.Node{h.Span(h.Class("status-pending"), g.Text("?")), g.Text(" Unknown")})
	case "ENABLED":
		return g.Group([]g.Node{h.Span(h.Class("status-success"), g.Text("✓")), g.Text(" Enabled")})
	case "OVERRIDDEN":
		return g.Group([]g.Node{h.Span(h.Class("status-disabled"), g.Text("⊘")), g.Text(" Overridden")})
	default:
		return g.Text(status)
	}
}

func resourceLinkNode(e *audit.Entry) g.Node {
	return auditlog.ResourceLink(e)
}
