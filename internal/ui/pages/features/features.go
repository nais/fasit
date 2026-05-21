package features

import (
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
			status := "UNKNOWN"
			if statuses, err := deployment.ListDeploymentStatuses(r.Context(), dep.ID); err == nil && len(statuses) > 0 {
				status = string(statuses[0].State)
			}
			depRows = append(depRows, depRow{
				FeatureName: dep.Feature.Name,
				Version:     dep.Feature.Version,
				Labels:      dep.TargetLabels,
				Status:      status,
				Created:     dep.Created,
				DepID:       dep.ID.String(),
			})
		}

		audits, _ := audit.ListRecent(r.Context(), 10)

		sort.Slice(depRows, func(i, j int) bool {
			return depRows[i].Created.After(depRows[j].Created)
		})
		depRows = depRows[:min(10, len(depRows))]

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
		renderPage(w, r, layout.Props{Title: data.CurrentFeature.Name, CurrentPage: components.PageFeatures, Content: detailPage(data)})
	}
}

func listPage(features []view.FeatureNav, deps []depRow, audits []*audit.Entry) g.Node {
	return h.Div(h.Class("container"),
		components.FeaturesSidebar(features, ""),
		h.Main(h.Class("main-content"),
			h.Div(h.Class("card"), h.Div(h.Class("card-body card-compact"),
				recentDeployments(deps),
			)),
			h.Div(h.Class("card"), h.Div(h.Class("card-body card-compact"),
				recentActivity(audits),
			)),
		),
	)
}

type depRow struct {
	FeatureName string
	Version     string
	Labels      map[string]string
	Status      string
	Created     time.Time
	DepID       string
}

func recentDeployments(rows []depRow) g.Node {
	if len(rows) == 0 {
		return h.P(h.Class("text-muted"), g.Text("No deployments."))
	}

	tableRows := make([]g.Node, 0, len(rows))
	for _, r := range rows {
		tableRows = append(tableRows, h.Tr(
			h.Td(h.A(h.Href("/features/"+r.FeatureName), g.Text(r.FeatureName))),
			h.Td(depLabelPills(r.Labels)),
			h.Td(h.A(h.Href("/deployments/"+r.DepID), g.Text(r.Version))),
			h.Td(renderStatus(r.Status)),
			h.Td(h.Class("text-muted text-right"), g.Text(view.RelativeTime(r.Created))),
		))
	}

	return g.Group([]g.Node{
		h.H3(g.Text("Recent deployments")),
		h.Table(h.Class("table table-compact"),
			h.THead(h.Tr(
				h.Th(g.Text("Feature")),
				h.Th(g.Text("Target")),
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
		env := envLinkNode(a)
		resource := resourceLinkNode(a)
		tableRows = append(tableRows, h.Tr(
			h.Td(g.Text(string(a.Action))),
			h.Td(resource),
			h.Td(env),
			h.Td(h.Class("text-muted"), g.Text(a.Description)),
			h.Td(g.Text(a.Actor)),
			h.Td(h.Class("text-muted"), g.Text(view.RelativeTime(a.CreatedAt))),
		))
	}

	return g.Group([]g.Node{
		h.H3(g.Text("Recent activity")),
		h.Table(h.Class("table table-compact"),
			h.THead(h.Tr(
				h.Th(g.Text("Action")),
				h.Th(g.Text("Resource")),
				h.Th(g.Text("Environment")),
				h.Th(g.Text("Detail")),
				h.Th(g.Text("Actor")),
				h.Th(g.Text("When")),
			)),
			h.TBody(g.Group(tableRows)),
		),
		h.Div(h.Style("margin-top: 0.75rem; font-size: 0.85rem;"), h.A(h.Href("/auditlog"), h.Class("link-muted"), g.Text("All activity →"))),
	})
}

func depLabelPills(labels map[string]string) g.Node {
	if len(labels) == 0 {
		return h.Span(h.Class("label-filter-tag"), g.Text("all environments"))
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

func detailPage(data *DetailPage) g.Node {
	content := deploymentDetailContent(data)
	return h.Div(h.Class("container"),
		components.FeaturesSidebar(data.Features, data.CurrentFeature.Name),
		h.Main(h.Class("main-content"),
			components.Breadcrumbs(data.Breadcrumbs),
			h.Div(h.Class("card"), h.Div(h.Class("card-body"), content)),
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
	label := e.ObjectType.Display() + " " + e.ObjectID
	var href string
	switch e.ObjectType {
	case audit.ObjectTypeFeature, audit.ObjectTypeDeployment:
		href = "/features/" + e.ObjectID
	case audit.ObjectTypeConfiguration:
		if i := strings.IndexByte(e.ObjectID, '/'); i > 0 {
			href = "/features/" + e.ObjectID[:i]
		}
	case audit.ObjectTypeEnvironment, audit.ObjectTypeEnvironmentValue:
		if e.TenantName != "" && e.EnvironmentName != "" {
			href = "/tenants/" + e.TenantName + "/envs/" + e.EnvironmentName
		}
	}
	if href == "" {
		return g.Text(label)
	}
	return h.A(h.Href(href), g.Text(label))
}

func envLinkNode(e *audit.Entry) g.Node {
	if e.TenantName == "" || e.EnvironmentName == "" {
		return g.Text("")
	}
	label := e.TenantName + "/" + e.EnvironmentName
	return h.A(h.Href("/tenants/"+e.TenantName+"/envs/"+e.EnvironmentName), g.Text(label))
}
