package features

import (
	"fmt"
	"net/http"
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
	Prefs          ViewPrefs
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
				})
			} else {
				for _, s := range statuses {
					depRows = append(depRows, depRow{
						FeatureName: dep.Feature.Name,
						Version:     dep.Feature.Version,
						Status:      string(s.State),
						Created:     dep.Created,
						DepID:       dep.ID.String(),
					})
				}
			}
		}

		audits, _ := audit.ListRecent(r.Context(), 10)

		sort.Slice(depRows, func(i, j int) bool {
			return depRows[i].Created.After(depRows[j].Created)
		})

		renderPage(w, r, layout.Props{Title: "Features", CurrentPage: components.PageFeatures, Content: listPage(toFeatureNavs(features), depRows, audits)})
	}
}

func Handler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := loadFeatureData(r)
		if err != nil {
			http.Error(w, "Failed to load feature data", http.StatusInternalServerError)
			return
		}
		data.Prefs = parseViewPrefs(r)
		data.ActiveTab = "overview"
		renderPage(w, r, layout.Props{Title: data.CurrentFeature.Name, CurrentPage: components.PageFeatures, Content: detailPage(data)})
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
		components.FeaturesSidebar(features, ""),
		h.Main(h.Class("main-content"),
			components.CardCompact(
				recentDeployments(deps),
			),
			components.CardCompact(
				recentActivity(audits),
			),
		),
	)
}

type depRow struct {
	FeatureName string
	Version     string
	Status      string
	Created     time.Time
	DepID       string
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
	}
	seen := make(map[aggKey]*aggRow)
	var ordered []aggKey
	for _, r := range rows {
		k := aggKey{r.FeatureName, r.Version}
		if agg, ok := seen[k]; ok {
			agg.Statuses = append(agg.Statuses, r.Status)
			if r.Created.After(agg.Latest) {
				agg.Latest = r.Created
			}
		} else {
			seen[k] = &aggRow{
				FeatureName: r.FeatureName,
				Version:     r.Version,
				Statuses:    []string{r.Status},
				Latest:      r.Created,
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
				h.Th(h.Class("text-right"), g.Text("When")),
			)),
			h.TBody(g.Group(tableRows)),
		),
		h.Div(h.Style("margin-top: 0.75rem; font-size: 0.85rem;"), h.A(h.Href("/deployments"), h.Class("link-muted"), g.Text("All deployments →"))),
	})
}

func recentActivity(audits []*audit.Entry) g.Node {
	if len(audits) == 0 {
		return h.P(h.Class("text-muted"), g.Text("No recent activity."))
	}

	tableRows := make([]g.Node, 0, len(audits))
	for _, a := range audits {
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
		case "DEPLOYED":
			deployed++
		case "PENDING", "PENDING-INSTALL", "PENDING-UPGRADE", "PENDING-ROLLBACK", "CREATED":
			pending++
		}
	}
	if deployed == total {
		return aggStatus{"status-success", "DEPLOYED"}
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

func featureTabs(featureName string) []components.Tab {
	return []components.Tab{
		{ID: "overview", Href: "/features/" + featureName, Label: "Overview"},
		{ID: "config", Href: "/features/" + featureName + "/config", Label: "Config"},
		{ID: "config-explorer", Href: "/features/" + featureName + "/config-explorer", Label: "Config Explorer"},
	}
}

func detailPage(data *DetailPage) g.Node {
	var content g.Node
	switch data.ActiveTab {
	case "config":
		content = globalConfigContent(data)
	case "config-explorer":
		content = configExplorerContent(data.CurrentFeature.Name, data.ExplorerData)
	default:
		content = deploymentDetailContent(data)
	}
	return h.Div(h.Class("container"),
		components.FeaturesSidebar(data.Features, data.CurrentFeature.Name),
		h.Main(h.Class("main-content"),
			components.Breadcrumbs(data.Breadcrumbs),
			components.Card(
				components.TabsNav(data.ActiveTab, featureTabs(data.CurrentFeature.Name)),
				content,
			),
		),
	)
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
	featureCrumb.Subtitle = feature.Description
	featureCrumb.SourceURL = feature.Source
	data := &DetailPage{
		Breadcrumbs:    []breadcrumb.Crumb{breadcrumb.Features(), featureCrumb},
		Features:       toFeatureNavs(features),
		CurrentFeature: feature,
	}
	loadDeploymentData(r.Context(), feature, data)
	return data, nil
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
	case "OVERRIDDEN":
		return g.Group([]g.Node{h.Span(h.Class("status-disabled"), g.Text("⊘")), g.Text(" OVERRIDDEN")})
	default:
		return g.Text(status)
	}
}

func resourceLinkNode(e *audit.Entry) g.Node {
	return auditlog.ResourceLink(e)
}
