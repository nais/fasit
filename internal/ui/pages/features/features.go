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
	"github.com/nais/fasit/internal/ui/uidata"
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

	IsAssignmentDetail     bool
	Assignment             *featureassignment.FeatureAssignment
	AssignmentStatusRows   []reconcileStatusRow
	AssignmentInstructions []*uidata.DeployInstruction
	AssignmentMatching     []matchingAssignment
	AssignmentSupersededBy *matchingAssignment
}

func ListHandler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deploys, _ := reconciler.ListRecentDeploys(r.Context(), 10)
		audits, _ := audit.ListRecent(r.Context(), 200)
		assignmentActors := assignmentActorsByID(audits)

		rows := make([]deployRow, 0, len(deploys))
		for _, d := range deploys {
			rows = append(rows, deployRow{
				FeatureName: d.FeatureName,
				Version:     d.FeatureVersion,
				Total:       d.Total,
				Deployed:    d.Deployed,
				Failed:      d.Failed,
				Pending:     d.Pending,
				When:        d.LastDeploy,
				Actor:       assignmentActors[d.FeatureAssignmentID.String()],
			})
		}

		renderPage(w, r, layout.Props{Title: "Home", CurrentPage: components.PageHome, Content: listPage(rows, audits), HideHeaderSearch: true})
	}
}

func IndexHandler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		features, err := featurepkg.ListActiveFeatures(r.Context())
		if err != nil {
			http.Error(w, "Failed to load features", http.StatusInternalServerError)
			return
		}

		query := strings.TrimSpace(r.URL.Query().Get("q"))
		rows := make([]featurepkg.FeatureSummary, 0, len(features))
		for _, feature := range features {
			if query == "" || featureIndexMatches(feature, query) {
				rows = append(rows, feature)
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
		data.Breadcrumbs = append(data.Breadcrumbs, breadcrumb.Crumb{Label: "Assignments"})
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
		data.Breadcrumbs = append(data.Breadcrumbs, breadcrumb.Crumb{Label: "Config"})
		data.ConfigItems, err = loadGlobalConfigItems(r.Context(), data.CurrentFeature)
		if err != nil {
			http.Error(w, "Failed to load config", http.StatusInternalServerError)
			return
		}
		data.RecentActivity, _ = audit.ListGlobalConfigForFeature(r.Context(), data.CurrentFeature.Name, 10)
		renderPage(w, r, layout.Props{Title: data.CurrentFeature.Name + " · Config", CurrentPage: components.PageFeatures, Content: detailPage(data)})
	}
}

func listPage(rows []deployRow, audits []*audit.Entry) g.Node {
	return h.Div(
		h.Class("container"),
		h.Main(
			h.Class("main-content landing-page"),
			landingSearch(),
			components.CardCompact(
				recentAssignments(rows),
			),
			components.CardCompact(
				recentActivity(audits),
			),
		),
	)
}

func landingSearch() g.Node {
	return h.Section(
		h.Class("landing-search"),
		h.Form(
			h.Method("get"), h.Action("/features"), h.Class("feature-search-form landing-search-form"), g.Attr("data-feature-search", ""),
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

func featureIndexPage(features []featurepkg.FeatureSummary, query string) g.Node {
	return h.Div(
		h.Class("container"),
		components.Breadcrumbs([]breadcrumb.Crumb{breadcrumb.Features()}),
		h.Main(
			h.Class("main-content"),
			h.Div(h.Class("card"), h.Div(
				h.Class("card-body"),
				h.Div(
					h.Class("assignments-header"),
					h.Div(
						h.Class("assignments-toolbar"),
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

func featureIndexMatches(feature featurepkg.FeatureSummary, query string) bool {
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

func featureIndexTable(features []featurepkg.FeatureSummary) g.Node {
	rows := make([]g.Node, 0, len(features))
	for i, feature := range features {
		rows = append(rows, h.Tr(
			h.Td(h.A(h.Href("/features/"+feature.Name), g.Text(feature.Name))),
			h.Td(h.Class("text-muted"), g.Text(feature.Description)),
			h.Td(h.Class("table-kebab-cell"), featureRowKebab(feature, i)),
		))
	}
	return h.Table(
		h.ID("feature-index-table"), h.Class("table sortable"),
		h.THead(h.Tr(
			h.Th(g.Text("Feature")),
			h.Th(g.Attr("data-no-sort", ""), g.Text("Description")),
			h.Th(g.Attr("data-no-sort", ""), h.Class("table-kebab-cell")),
		)),
		h.TBody(g.Group(rows)),
	)
}

func featureRowKebab(feature featurepkg.FeatureSummary, idx int) g.Node {
	if feature.Source == "" {
		return g.Text("")
	}
	kebabID := fmt.Sprintf("feature-kebab-%d", idx)
	return components.KebabWrap(kebabID, []g.Node{
		h.A(h.Href(feature.Source), h.Class("kebab-item"), g.Attr("target", "_blank"), g.Attr("rel", "noopener noreferrer"), g.Text("View on GitHub")),
	})
}

type deployRow struct {
	FeatureName string
	Version     string
	Total       int
	Deployed    int
	Failed      int
	Pending     int
	When        time.Time
	Actor       string
}

func assignmentActorsByID(audits []*audit.Entry) map[string]string {
	ret := make(map[string]string)
	for _, a := range audits {
		if a.ObjectType != audit.ObjectTypeFeatureAssignment || a.Action != audit.ActionCreated || len(a.Metadata) == 0 {
			continue
		}
		var metadata struct {
			FeatureAssignmentID string `json:"assignmentId"`
		}
		if err := json.Unmarshal(a.Metadata, &metadata); err != nil || metadata.FeatureAssignmentID == "" {
			continue
		}
		ret[metadata.FeatureAssignmentID] = a.Actor
	}
	return ret
}

func recentAssignments(rows []deployRow) g.Node {
	if len(rows) == 0 {
		return h.P(h.Class("text-muted"), g.Text("No deployments."))
	}

	tableRows := make([]g.Node, 0, len(rows))
	for i, r := range rows {
		tableRows = append(tableRows, h.Tr(
			h.Td(h.A(h.Href("/features/"+r.FeatureName), g.Text(r.FeatureName))),
			h.Td(h.Class("text-muted"), g.Text(r.Version)),
			h.Td(deployRollupStatus(r)),
			h.Td(view.ActorName(r.Actor)),
			h.Td(h.Class("text-muted text-right"), h.Title(view.FormatTime(r.When)), g.Text(view.RelativeTime(r.When))),
			h.Td(h.Class("table-kebab-cell"), assignmentRowKebab(r.Actor, i)),
		))
	}

	return g.Group([]g.Node{
		h.H3(g.Text("Recent deployments")),
		h.Table(
			h.Class("table table-compact"),
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

// deployRollupStatus renders a feature version's rollout across environments:
// a green check when every instance is deployed, otherwise a breakdown of the
// deployed/failed/pending counts coloured by the most severe outcome.
func deployRollupStatus(r deployRow) g.Node {
	if r.Total > 0 && r.Deployed == r.Total {
		return g.Group([]g.Node{h.Span(h.Class("status-success"), g.Text("\u2713")), g.Text(" Deployed")})
	}
	var parts []string
	if r.Deployed > 0 {
		parts = append(parts, fmt.Sprintf("%d deployed", r.Deployed))
	}
	if r.Failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", r.Failed))
	}
	if r.Pending > 0 {
		parts = append(parts, fmt.Sprintf("%d pending", r.Pending))
	}
	if other := r.Total - r.Deployed - r.Failed - r.Pending; other > 0 {
		parts = append(parts, fmt.Sprintf("%d unknown", other))
	}
	class, icon := "status-pending", "\u23f3"
	if r.Failed > 0 {
		class, icon = "status-error", "\u2717"
	}
	return g.Group([]g.Node{h.Span(h.Class(class), g.Text(icon)), g.Text(" " + strings.Join(parts, ", "))})
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

	return g.Group([]g.Node{
		h.H3(g.Text("Recent activity")),
		auditview.ActivityTable(filtered, ""),
		h.Div(h.Class("section-link-row"), h.A(h.Href("/auditlog"), h.Class("link-muted"), g.Text("All activity →"))),
	})
}

func detailPage(data *DetailPage) g.Node {
	var content g.Node
	var breadcrumbActions []g.Node
	var rightSidebar g.Node
	switch data.ActiveTab {
	case "assignments":
		if data.IsAssignmentDetail {
			content = assignmentDetailPageContent(data)
		} else {
			content = assignmentSpecsContent(data)
		}
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
		rightSidebar = h.Aside(
			h.Class("right-sidebar"),
			components.CardCompact(auditview.ActivityList(auditview.ActivityListParams{
				Title:        title,
				AllHref:      "/auditlog?q=" + url.QueryEscape(data.CurrentFeature.Name),
				Entries:      data.RecentActivity,
				ResourceNode: featureActivityResourceNode,
			})),
		)
	}
	return h.Div(
		h.Class("container"),
		featureSidebar(data),
		components.Breadcrumbs(data.Breadcrumbs, breadcrumbActions...),
		h.Main(
			h.Class("main-content"),
			components.Card(content),
		),
		g.If(rightSidebar != nil, rightSidebar),
	)
}

func featureSidebar(data *DetailPage) g.Node {
	return components.FeatureSidebar(data.CurrentFeature.Name, data.ActiveTab, "", "", data.FeatureEnvs)
}

// featureActivityResourceNode renders the activity-sidebar resource for a
// feature-scoped page: config keys without the feature prefix, and assignment
// events as a bare "assignment" link (the feature is already the page context;
// the version and target are shown in the entry description).
func featureActivityResourceNode(e *audit.Entry) g.Node {
	switch e.ObjectType {
	case audit.ObjectTypeConfiguration:
		if i := strings.IndexByte(e.ObjectID, '/'); i > 0 {
			return h.Code(g.Text(e.ObjectID[i+1:]))
		}
	case audit.ObjectTypeFeatureAssignment:
		if href := auditview.AssignmentHref(e); href != "" {
			return h.A(h.Href(href), g.Text("assignment"))
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
