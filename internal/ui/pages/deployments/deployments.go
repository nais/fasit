package deployments

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type featureGroup struct {
	FeatureName string
	LatestTime  time.Time
	Targets     []Summary
}

func groupDeployments(items []Summary) []featureGroup {
	type key struct{ feature, target string }
	latest := make(map[key]Summary)
	for _, item := range items {
		k := key{item.FeatureName, item.Target}
		if existing, ok := latest[k]; !ok || item.createdAt.After(existing.createdAt) {
			latest[k] = item
		}
	}

	groupMap := make(map[string]*featureGroup)
	for _, item := range latest {
		fg, ok := groupMap[item.FeatureName]
		if !ok {
			groupMap[item.FeatureName] = &featureGroup{FeatureName: item.FeatureName, LatestTime: item.createdAt, Targets: []Summary{item}}
			continue
		}
		fg.Targets = append(fg.Targets, item)
		if item.createdAt.After(fg.LatestTime) {
			fg.LatestTime = item.createdAt
		}
	}

	groups := make([]featureGroup, 0, len(groupMap))
	for _, fg := range groupMap {
		sort.Slice(fg.Targets, func(i, j int) bool {
			return fg.Targets[i].createdAt.After(fg.Targets[j].createdAt)
		})
		groups = append(groups, *fg)
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].LatestTime.After(groups[j].LatestTime)
	})
	return groups
}

func ListHandler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deps, _ := deployment.ListDeployments(r.Context())

		showAll := r.URL.Query().Get("show") == "all"

		rows := make([]deploymentRow, 0, len(deps))
		for _, dep := range deps {
			statuses, _ := deployment.ListDeploymentStatuses(r.Context(), dep.ID)
			state, _ := deployment.AggregateState(statuses)

			rows = append(rows, deploymentRow{
				FeatureName:  dep.Feature.Name,
				Version:      dep.Feature.Version,
				Target:       deploymentTarget(dep),
				TargetLabels: deploymentTargetLabels(dep),
				Status:       string(state),
				Active:       dep.Active,
				Created:      dep.Created,
				DeploymentID: dep.ID.String(),
			})
		}

		sort.Slice(rows, func(i, j int) bool {
			return rows[i].Created.After(rows[j].Created)
		})

		renderPage(w, r, layout.Props{
			Title:       "Deployments",
			CurrentPage: components.PageDeployments,
			Content:     listPage(rows, showAll),
		})
	}
}

type deploymentRow struct {
	FeatureName  string
	Version      string
	Target       string
	TargetLabels map[string]string
	Status       string
	Active       bool
	Created      time.Time
	DeploymentID string
}

func listPage(rows []deploymentRow, showAll bool) g.Node {
	var toggleLink g.Node
	if showAll {
		toggleLink = h.A(h.Href("/deployments"), h.Class("toggle-pill active"), g.Text("Show inactive"))
	} else {
		toggleLink = h.A(h.Href("/deployments?show=all"), h.Class("toggle-pill"), g.Text("Show inactive"))
	}

	var filtered []deploymentRow
	for _, r := range rows {
		if showAll || r.Active {
			filtered = append(filtered, r)
		}
	}

	return h.Div(
		h.Class("container"),
		h.Main(
			h.Class("main-content"),
			components.Breadcrumbs(nil),
			h.Div(h.Class("card"), h.Div(h.Class("card-body"),
				h.Div(h.Class("deployments-header"),
					h.Div(h.Class("deployments-toolbar"),
						h.Input(
							h.Type("search"),
							h.Class("table-filter"),
							h.Placeholder("Filter by feature, target, version…"),
							g.Attr("aria-label", "Filter deployments"),
							g.Attr("autocomplete", "off"),
							g.Attr("data-filter-table", "deployments-table"),
						),
						toggleLink,
					),
					h.Button(h.Type("button"), h.Class("btn-small"), g.Attr("popovertarget", "new-deployment"), g.Text("+ New deployment")),
					newDeploymentPopover(),
				),
				deploymentsTable(filtered),
			)),
		),
	)
}

func deploymentsTable(rows []deploymentRow) g.Node {
	if len(rows) == 0 {
		return h.P(h.Class("text-muted"), g.Text("No deployments."))
	}

	tableRows := make([]g.Node, 0, len(rows))
	for _, r := range rows {
		rowClass := ""
		if !r.Active {
			rowClass = "deployment-inactive"
		}

		var statusNode g.Node
		if !r.Active {
			statusNode = g.Group([]g.Node{h.Span(h.Class("status-disabled"), g.Text("○")), g.Text(" INACTIVE")})
		} else {
			statusNode = rolloutStatus(r.Status)
		}

		tableRows = append(tableRows, h.Tr(
			g.If(rowClass != "", h.Class(rowClass)),
			h.Td(h.A(h.Href("/features/"+r.FeatureName), g.Text(r.FeatureName))),
			h.Td(targetPills(r.TargetLabels)),
			h.Td(h.A(h.Href("/deployments/"+r.DeploymentID), g.Text(r.Version))),
			h.Td(statusNode),
			h.Td(g.Attr("data-sort-value", r.Created.Format(time.RFC3339)), g.Text(view.RelativeTime(r.Created))),
		))
	}

	return h.Table(
		h.Class("table sortable"),
		h.ID("deployments-table"),
		g.Attr("data-sort-key", "deployments-list"),
		h.THead(h.Tr(
			h.Th(g.Text("Feature")),
			h.Th(g.Text("Target"), g.Attr("data-no-sort", "")),
			h.Th(g.Text("Version")),
			h.Th(g.Text("Status")),
			h.Th(g.Text("Updated")),
		)),
		h.TBody(g.Group(tableRows)),
	)
}

func newDeploymentPopover() g.Node {
	return h.Div(g.Attr("popover", ""), h.ID("new-deployment"),
		h.H3(g.Text("New deployment")),
		h.Form(h.Method("POST"), h.Action("/deployments"),
			h.Label(g.Text("Feature")),
			h.Input(h.Type("text"), h.Name("feature_name"), g.Attr("required", ""), g.Attr("placeholder", "e.g. naiserator")),
			h.Label(g.Text("Chart")),
			h.Input(h.Type("text"), h.Name("chart"), g.Attr("required", ""), g.Attr("placeholder", "e.g. oci://naiserator")),
			h.Label(g.Text("Version")),
			h.Input(h.Type("text"), h.Name("version"), g.Attr("required", ""), g.Attr("placeholder", "e.g. 2026-05-15-001")),
			h.Label(g.Text("Target labels (one per line, key=value)")),
			h.Textarea(h.Name("target_labels_raw"), g.Attr("rows", "3"), g.Attr("placeholder", "kind=tenant\ntenant=nav")),
			h.Div(h.Class("popover-actions"),
				h.Button(h.Type("submit"), g.Text("Deploy")),
				h.Button(h.Type("button"), g.Attr("popovertarget", "new-deployment"), g.Attr("popovertargetaction", "hide"), g.Text("Cancel")),
			),
		),
	)
}

func deploymentTarget(dep *deployment.Deployment) string {
	labels := dep.Target()
	if len(labels) == 0 {
		return "All environments"
	}

	parts := make([]string, 0, len(labels))
	for _, label := range labels {
		parts = append(parts, label.Key+"="+label.Value)
	}

	return strings.Join(parts, ", ")
}

func deploymentTargetLabels(dep *deployment.Deployment) map[string]string {
	labels := dep.Target()
	if len(labels) == 0 {
		return nil
	}
	out := make(map[string]string, len(labels))
	for _, label := range labels {
		out[label.Key] = label.Value
	}
	return out
}
