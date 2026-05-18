package features

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/database"
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
	Breadcrumbs       []breadcrumb.Crumb
	Features          []view.FeatureNav
	CurrentFeature    *Feature
	ChartDescriptions []string
	DeploymentEnvs    []DeploymentEnvStatus
	RolloutEnvs       []RolloutEnvStatus
	Rollouts          []RolloutItem
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

func ListHandler(renderPage RenderPage, repo database.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		features, err := featurepkg.Features(r.Context())
		if err != nil {
			http.Error(w, "Failed to load features", http.StatusInternalServerError)
			return
		}
		failed, pending := featureStatusCounts(r.Context(), repo)
		navs := toFeatureNavs(features, failed, pending)

		audits, _ := audit.RecentAudits(r.Context(), 10)
		deps, _ := deployment.ListDeployments(r.Context())

		var rows []depRow
		for _, dep := range deps {
			status := "UNKNOWN"
			if statuses, err := deployment.ListDeploymentStatuses(r.Context(), dep.ID); err == nil {
				state, _ := deployment.AggregateState(statuses)
				status = string(state)
			}
			rows = append(rows, depRow{
				FeatureName: dep.Feature.Name,
				Version:     dep.Feature.Version,
				Labels:      dep.TargetLabels,
				Status:      status,
				Active:      dep.Active,
				Created:     dep.Created,
				DepID:       dep.ID.String(),
			})
		}

		renderPage(w, r, layout.Props{Title: "Features", CurrentPage: components.PageFeatures, Content: listPage(navs, rows, audits)})
	}
}

func Handler(renderPage RenderPage, repo database.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := loadFeatureData(r, repo)
		if err != nil {
			http.Error(w, "Failed to load feature data", http.StatusInternalServerError)
			return
		}
		renderPage(w, r, layout.Props{Title: data.CurrentFeature.Name, CurrentPage: components.PageFeatures, Content: detailPage(data)})
	}
}

func listPage(features []view.FeatureNav, deps []depRow, audits []*model.AuditLog) g.Node {
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
	Active      bool
	Created     time.Time
	DepID       string
}

type depGroup struct {
	FeatureName string
	Rows        []depRow
}

func recentDeployments(rows []depRow) g.Node {
	if len(rows) == 0 {
		return h.P(h.Class("text-muted"), g.Text("No deployments."))
	}

	groupMap := map[string]*depGroup{}
	var order []string
	for _, r := range rows {
		grp, ok := groupMap[r.FeatureName]
		if !ok {
			grp = &depGroup{FeatureName: r.FeatureName}
			groupMap[r.FeatureName] = grp
			order = append(order, r.FeatureName)
		}
		grp.Rows = append(grp.Rows, r)
	}

	if len(order) > 5 {
		order = order[:5]
	}

	groups := make([]g.Node, 0, len(order))
	for _, name := range order {
		grp := groupMap[name]
		tableRows := make([]g.Node, 0, len(grp.Rows))
		for _, r := range grp.Rows {
			rowClass := ""
			if !r.Active {
				rowClass = "deployment-inactive"
			}
			tableRows = append(tableRows, h.Tr(g.If(rowClass != "", h.Class(rowClass)),
				h.Td(labelPills(r.Labels)),
				h.Td(h.A(h.Href("/deployments/"+r.DepID), g.Text(r.Version))),
				h.Td(rolloutStatus(r.Status)),
				h.Td(h.Class("text-muted"), g.Text(view.RelativeTime(r.Created))),
			))
		}

		groups = append(groups, h.Details(h.Class("deployment-group"),
			h.Summary(h.Class("deployment-group-summary"),
				h.Span(h.Class("dep-feature-toggle")),
				h.A(h.Href("/features/"+name), g.Text(name)),
				aggregateStatus(grp.Rows),
				h.Span(h.Class("text-muted dep-group-count"), g.Textf("%d targets", len(grp.Rows))),
			),
			h.Div(h.Class("deployment-group-body-table"),
				h.Table(h.Class("table table-compact"),
					h.THead(h.Tr(
						h.Th(g.Text("Target")),
						h.Th(g.Text("Version")),
						h.Th(g.Text("Status")),
						h.Th(g.Text("Updated")),
					)),
					h.TBody(g.Group(tableRows)),
				),
			),
		))
	}

	return g.Group([]g.Node{
		h.H3(g.Text("Recent deployments")),
		h.Div(h.Class("deployments-list"), g.Group(groups)),
	})
}

func aggregateStatus(rows []depRow) g.Node {
	failed, pending, deployed := 0, 0, 0
	for _, r := range rows {
		switch strings.ToUpper(r.Status) {
		case "FAILED":
			failed++
		case "PENDING", "CREATED", "UNKNOWN":
			pending++
		case "DEPLOYED":
			deployed++
		}
	}
	switch {
	case failed > 0:
		return h.Span(h.Class("status-badge status-error"), g.Textf("%d failed", failed))
	case pending > 0:
		return h.Span(h.Class("status-badge status-pending"), g.Textf("%d pending", pending))
	case deployed == len(rows):
		return h.Span(h.Class("status-badge status-success"), g.Text("all deployed"))
	default:
		return nil
	}
}

func recentActivity(audits []*model.AuditLog) g.Node {
	if len(audits) == 0 {
		return h.P(h.Class("text-muted"), g.Text("No recent activity."))
	}

	tableRows := make([]g.Node, 0, len(audits))
	for _, a := range audits {
		tableRows = append(tableRows, h.Tr(
			h.Td(g.Text(a.Description)),
			h.Td(g.Text(a.Actor)),
			h.Td(h.Class("text-muted"), g.Text(view.RelativeTime(a.CreatedAt))),
		))
	}

	return g.Group([]g.Node{
		h.H3(g.Text("Recent activity")),
		h.Table(h.Class("table table-compact"),
			h.THead(h.Tr(
				h.Th(g.Text("Action")),
				h.Th(g.Text("Actor")),
				h.Th(g.Text("When")),
			)),
			h.TBody(g.Group(tableRows)),
		),
	})
}

func detailPage(data *DetailPage) g.Node {
	var content g.Node
	if data.CurrentFeature.HasDeployments {
		content = deploymentDetailContent(data)
	} else {
		content = rolloutDetailContent(data)
	}
	return h.Div(h.Class("container"),
		components.FeaturesSidebar(data.Features, data.CurrentFeature.Name),
		h.Main(h.Class("main-content"),
			components.Breadcrumbs(data.Breadcrumbs),
			h.Div(h.Class("card"), h.Div(h.Class("card-body"), chartInfoHeader(data))),
			h.Div(h.Class("card"), h.Div(h.Class("card-body"), content)),
		),
	)
}

func chartInfoHeader(data *DetailPage) g.Node {
	feat := data.CurrentFeature.Feature
	rows := []g.Node{}
	if feat.Chart != "" {
		rows = append(rows, metaRow("Chart", g.Text(feat.Chart)))
	}
	if feat.Source != "" {
		rows = append(rows, metaRow("Source", h.A(h.Href(feat.Source), h.Target("_blank"), g.Text(feat.Source))))
	}
	for i, desc := range data.ChartDescriptions {
		label := "Description"
		if len(data.ChartDescriptions) > 1 {
			label = fmt.Sprintf("Description %d", i+1)
		}
		rows = append(rows, metaRow(label, g.Text(desc)))
	}
	return h.Table(h.Class("table meta-table"), h.TBody(rows...))
}

func metaRow(label string, value g.Node) g.Node {
	return h.Tr(h.Td(h.Class("th-like"), g.Text(label)), h.Td(value))
}

func loadFeatureData(r *http.Request, repo database.Repo) (*DetailPage, error) {
	featureName := chi.URLParam(r, "feature")
	features, err := featurepkg.Features(r.Context())
	if err != nil {
		return nil, err
	}
	feature, err := featurepkg.FeatureByName(r.Context(), featureName)
	if err != nil {
		return nil, err
	}
	failed, pending := featureStatusCounts(r.Context(), repo)
	data := &DetailPage{
		Breadcrumbs:    []breadcrumb.Crumb{breadcrumb.Features(), breadcrumb.Feature(featureName)},
		Features:       toFeatureNavs(features, failed, pending),
		CurrentFeature: &Feature{Feature: feature, Config: featureConfigItems(feature)},
	}
	if feature.HasDeployments {
		loadDeploymentData(r.Context(), repo, feature, data)
	} else {
		loadRolloutData(r.Context(), repo, feature, data)
	}
	data.ChartDescriptions = dedupedChartDescriptions(feature, data)
	return data, nil
}

func featureStatusCounts(_ context.Context, _ database.Repo) (failed, pending map[string]int) {
	return map[string]int{}, map[string]int{}
}

func dedupedChartDescriptions(feature *model.Feature, data *DetailPage) []string {
	seen := map[string]struct{}{}
	ret := []string{}
	add := func(d string) {
		d = strings.TrimSpace(d)
		if d == "" {
			return
		}
		if _, ok := seen[d]; ok {
			return
		}
		seen[d] = struct{}{}
		ret = append(ret, d)
	}
	for _, desc := range data.ChartDescriptions {
		add(desc)
	}
	add(feature.Description)
	return ret
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

func toFeatureNavs(features []*model.Feature, failedCounts, pendingCounts map[string]int) []view.FeatureNav {
	ret := make([]view.FeatureNav, 0, len(features))
	for _, feature := range features {
		ret = append(ret, view.FeatureNav{
			Name:         feature.Name,
			FailedCount:  failedCounts[feature.Name],
			PendingCount: pendingCounts[feature.Name],
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
	case "OVERRIDDEN":
		return g.Group([]g.Node{h.Span(h.Class("status-disabled"), g.Text("⊘")), g.Text(" OVERRIDDEN")})
	default:
		return g.Text(status)
	}
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
