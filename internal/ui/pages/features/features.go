package features

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/reconciler"
	"github.com/nais/fasit/internal/ui/auditview"
	"github.com/nais/fasit/internal/ui/breadcrumb"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/featureenvs"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, *http.Request, layout.Props)

type DetailPage struct {
	Breadcrumbs     []breadcrumb.Crumb
	Features        []view.FeatureNav
	CurrentFeature  *featurepkg.Feature
	FeatureEnvs     []featureenvs.Environment
	AssignmentEnvs  []AssignmentEnvStatus
	RecentActivity  []*audit.Entry
	ActiveTab       string
	ConfigItems     []components.ConfigItem
	Versions        []featurepkg.FeatureVersion
	VersionEnvs     map[string][]featureenvs.Environment
	IsVersionDetail bool
}

func ListHandler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fas, _ := featureassignment.ListRecent(r.Context())
		audits, _ := audit.ListRecent(r.Context(), 200)
		assignmentActors := assignmentActorsByID(audits)

		var assignmentRows []assignmentRow
		for _, fa := range fas {
			statuses, err := reconciler.ReconcileStatuses(r.Context(), fa.ID)
			if err != nil || len(statuses) == 0 {
				assignmentRows = append(assignmentRows, assignmentRow{
					FeatureName:  fa.Feature.Name,
					Version:      fa.Feature.Version,
					Status:       "UNKNOWN",
					Created:      fa.Created,
					AssignmentID: fa.ID.String(),
					Actor:        assignmentActors[fa.ID.String()],
				})
			} else {
				for _, s := range statuses {
					assignmentRows = append(assignmentRows, assignmentRow{
						FeatureName:  fa.Feature.Name,
						Version:      fa.Feature.Version,
						Status:       string(s.State),
						Created:      fa.Created,
						AssignmentID: fa.ID.String(),
						Actor:        assignmentActors[fa.ID.String()],
					})
				}
			}
		}

		sort.Slice(assignmentRows, func(i, j int) bool {
			return assignmentRows[i].Created.After(assignmentRows[j].Created)
		})

		renderPage(w, r, layout.Props{Title: "Home", CurrentPage: components.PageHome, Content: listPage(assignmentRows, audits), HideHeaderSearch: true})
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
			handleFeatureLoadError(w, r, err)
			return
		}
		data.ActiveTab = "overview"
		data.RecentActivity, _ = audit.ListForFeature(r.Context(), data.CurrentFeature.Name, 10)
		renderPage(w, r, layout.Props{Title: data.CurrentFeature.Name, CurrentPage: components.PageFeatures, Content: detailPage(data)})
	}
}

func DeploySpecsHandler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := loadFeatureData(r)
		if err != nil {
			handleFeatureLoadError(w, r, err)
			return
		}
		data.ActiveTab = "assignments"
		data.RecentActivity, _ = audit.ListAssignmentsForFeature(r.Context(), data.CurrentFeature.Name, 10)
		renderPage(w, r, layout.Props{Title: data.CurrentFeature.Name + " · Deploy specs", CurrentPage: components.PageFeatures, Content: detailPage(data)})
	}
}

func ConfigTabHandler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := loadFeatureData(r)
		if err != nil {
			handleFeatureLoadError(w, r, err)
			return
		}
		data.ActiveTab = "config"
		data.ConfigItems, err = loadGlobalConfigItems(r.Context(), data.CurrentFeature)
		if err != nil {
			http.Error(w, "Failed to load config", http.StatusInternalServerError)
			return
		}
		data.RecentActivity, _ = audit.ListGlobalConfigForFeature(r.Context(), data.CurrentFeature.Name, 10)
		renderPage(w, r, layout.Props{Title: data.CurrentFeature.Name + " · Config", CurrentPage: components.PageFeatures, Content: detailPage(data)})
	}
}

func listPage(fas []assignmentRow, audits []*audit.Entry) g.Node {
	return h.Div(h.Class("container"),
		h.Main(h.Class("main-content landing-page"),
			landingSearch(),
			components.CardCompact(
				recentAssignments(fas),
			),
			components.CardCompact(
				recentActivity(audits),
			),
		),
	)
}

func landingSearch() g.Node {
	return h.Section(h.Class("landing-search"),
		h.Form(h.Method("get"), h.Action("/features"), h.Class("feature-search-form landing-search-form"), g.Attr("data-feature-search", ""),
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
		components.Breadcrumbs([]breadcrumb.Crumb{breadcrumb.Features()}),
		h.Main(h.Class("main-content"),
			h.Div(h.Class("card"), h.Div(h.Class("card-body"),
				h.Div(h.Class("assignments-header"),
					h.Div(h.Class("assignments-toolbar"),
						h.Input(
							h.Type("search"),
							h.Class("table-filter"),
							h.Name("q"),
							h.Placeholder("Filter features…"),
							g.Attr("aria-label", "Filter features"),
							g.Attr("autocomplete", "off"),
							g.Attr("data-url-filter", "q"),
							g.If(query != "", h.Value(query)),
						),
					),
				),
				featureIndexTable(features),
			)),
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
	for i, feature := range features {
		rows = append(rows, h.Tr(
			h.Td(h.A(h.Href("/features/"+feature.Name), g.Text(feature.Name))),
			h.Td(h.Class("text-muted"), g.Text(feature.Description)),
			h.Td(h.Class("table-kebab-cell"), featureRowKebab(feature, i)),
		))
	}
	return h.Table(h.ID("feature-index-table"), h.Class("table sortable"),
		h.THead(h.Tr(
			h.Th(g.Text("Feature")),
			h.Th(g.Attr("data-no-sort", ""), g.Text("Description")),
			h.Th(g.Attr("data-no-sort", ""), h.Class("table-kebab-cell")),
		)),
		h.TBody(g.Group(rows)),
	)
}

func featureRowKebab(feature featureIndexRow, idx int) g.Node {
	if feature.Source == "" {
		return g.Text("")
	}
	kebabID := fmt.Sprintf("feature-kebab-%d", idx)
	return components.KebabWrap(kebabID, []g.Node{
		h.A(h.Href(feature.Source), h.Class("kebab-item"), g.Attr("target", "_blank"), g.Attr("rel", "noopener noreferrer"), g.Text("View on GitHub")),
	})
}

type assignmentRow struct {
	FeatureName  string
	Version      string
	Status       string
	Created      time.Time
	AssignmentID string
	Actor        string
}

func assignmentActorsByID(audits []*audit.Entry) map[string]string {
	ret := make(map[string]string)
	for _, a := range audits {
		if a.ObjectType != audit.ObjectTypeFeatureAssignment || a.Action != audit.ActionCreated || len(a.Metadata) == 0 {
			continue
		}
		var metadata struct {
			FeatureAssignmentID string `json:"deploymentId"`
		}
		if err := json.Unmarshal(a.Metadata, &metadata); err != nil || metadata.FeatureAssignmentID == "" {
			continue
		}
		ret[metadata.FeatureAssignmentID] = a.Actor
	}
	return ret
}

func recentAssignments(rows []assignmentRow) g.Node {
	if len(rows) == 0 {
		return h.P(h.Class("text-muted"), g.Text("No assignments."))
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
	for i, k := range ordered {
		agg := seen[k]
		tableRows = append(tableRows, h.Tr(
			h.Td(h.A(h.Href("/features/"+agg.FeatureName), g.Text(agg.FeatureName))),
			h.Td(h.Class("text-muted"), g.Text(agg.Version)),
			h.Td(renderAggStatus(agg.Statuses)),
			h.Td(view.ActorName(agg.Actor)),
			h.Td(h.Class("text-muted text-right"), h.Title(view.FormatTime(agg.Latest)), g.Text(view.RelativeTime(agg.Latest))),
			h.Td(h.Class("table-kebab-cell"), assignmentRowKebab(agg.Actor, i)),
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
				h.Th(h.Class("table-kebab-cell")),
			)),
			h.TBody(g.Group(tableRows)),
		),
		h.Div(h.Class("section-link-row"), h.A(h.Href("/assignments"), h.Class("link-muted"), g.Text("All assignments →"))),
	})
}

func assignmentRowKebab(actor string, idx int) g.Node {
	href := view.ActorWorkflowURL(actor)
	if href == "" {
		return g.Text("")
	}
	kebabID := fmt.Sprintf("assignment-kebab-%d", idx)
	return components.KebabWrap(kebabID, []g.Node{
		h.A(h.Href(href), h.Class("kebab-item"), g.Attr("target", "_blank"), g.Attr("rel", "noopener noreferrer"), g.Text("View workflow run")),
	})
}

func recentActivity(audits []*audit.Entry) g.Node {
	filtered := make([]*audit.Entry, 0, len(audits))
	for _, a := range audits {
		if a.ObjectType == audit.ObjectTypeFeatureAssignment && a.Action == audit.ActionCreated {
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
		resource := auditview.ResourceLink(a)
		tableRows = append(tableRows, h.Tr(
			h.Td(g.Text(auditview.DisplayAction(a))),
			h.Td(resource),
			h.Td(h.Class("text-muted"), auditview.DetailNode(a)),
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
		h.Div(h.Class("section-link-row"), h.A(h.Href("/auditlog"), h.Class("link-muted"), g.Text("All activity →"))),
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
	var breadcrumbActions []g.Node
	var rightSidebar g.Node
	switch data.ActiveTab {
	case "assignments":
		content = assignmentSpecsContent(data)
	case "config":
		content = globalConfigContent(data)
	case "versions":
		if data.IsVersionDetail {
			content = versionDetailContent(data)
		} else {
			content = versionsListContent(data)
		}
	default:
		content = assignmentDetailContent(data)
	}
	if len(data.RecentActivity) > 0 {
		title := "Recent activity"
		switch data.ActiveTab {
		case "assignments":
			title = "Assignment activity"
		case "config":
			title = "Config activity"
		}
		rightSidebar = h.Aside(h.Class("right-sidebar"),
			components.CardCompact(auditview.ActivityList(auditview.ActivityListParams{
				Title:        title,
				AllHref:      "/auditlog?q=" + url.QueryEscape(data.CurrentFeature.Name),
				Entries:      data.RecentActivity,
				ResourceNode: configKeyNode,
			})),
		)
	}
	return h.Div(h.Class("container"),
		featureSidebar(data),
		components.Breadcrumbs(data.Breadcrumbs, breadcrumbActions...),
		h.Main(h.Class("main-content"),
			components.Card(content),
		),
		g.If(rightSidebar != nil, rightSidebar),
	)
}

func featureSidebar(data *DetailPage) g.Node {
	return components.FeatureSidebar(data.CurrentFeature.Name, data.ActiveTab, "", "", data.FeatureEnvs)
}

func configKeyNode(e *audit.Entry) g.Node {
	if e.ObjectType == audit.ObjectTypeConfiguration {
		if i := strings.IndexByte(e.ObjectID, '/'); i > 0 {
			return h.Code(g.Text(e.ObjectID[i+1:]))
		}
	}
	return auditview.ResourceLink(e)
}

func handleFeatureLoadError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		http.Redirect(w, r, "/features", http.StatusSeeOther)
		return
	}
	http.Error(w, "Failed to load feature data", http.StatusInternalServerError)
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
	loadAssignmentData(r.Context(), feature, data)
	data.FeatureEnvs = featureenvs.LoadEnvironments(r.Context(), feature)
	setFeatureBreadcrumbSubtitle(data)
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

func featureTargetsKind(kinds []environment.EnvironmentKind, envKind environment.EnvironmentKind) bool {
	if len(kinds) == 0 {
		return true
	}
	return slices.Contains(kinds, envKind)
}

func lastDeployedCell(t time.Time, class string) g.Node {
	attrs := []g.Node{}
	if class != "" {
		attrs = append(attrs, h.Class(class))
	}
	if t.IsZero() {
		attrs = append(attrs, h.Span(h.Class("text-muted"), g.Text("never")))
		return h.Td(attrs...)
	}
	attrs = append(attrs, h.Title(view.FormatTime(t)), g.Text(view.RelativeTime(t)))
	return h.Td(attrs...)
}
