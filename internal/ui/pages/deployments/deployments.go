package deployments

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/ui/breadcrumb"
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

func ListHandler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var items []Summary

		deployments, err := deployment.ListDeployments(r.Context())
		if err == nil {
			for _, dep := range deployments {
				statuses, _ := deployment.ListDeploymentStatuses(r.Context(), dep.ID)
				state, disabledCount := deployment.AggregateState(statuses)
				items = append(items, Summary{
					FeatureName:   dep.Feature.Name,
					Version:       dep.Feature.Version,
					Status:        string(state),
					Target:        deploymentTarget(dep),
					TargetLabels:  deploymentTargetLabels(dep),
					Created:       view.FormatTime(dep.Created),
					Completed:     latestStatusTime(statuses),
					DeploymentID:  dep.ID.String(),
					createdAt:     dep.Created,
					disabledCount: disabledCount,
				})
			}
		}

		groups := groupDeployments(items)

		renderPage(w, r, layout.Props{
			Title:       "Deployments",
			CurrentPage: components.PageDeployments,
			Content:     listPage(groups),
		})
	}
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

func listPage(groups []featureGroup) g.Node {
	return h.Div(
		h.Class("container"),
		h.Main(
			h.Class("main-content"),
			components.Breadcrumbs([]breadcrumb.Crumb{breadcrumb.Deployments()}),
			h.Div(
				h.Class("card"),
				h.Div(
					h.Class("card-body"),
					h.Div(
						h.Class("deployments-header"),
						h.H1(g.Text("Deployments")),
						g.If(len(groups) > 0, h.Button(
							h.Type("button"),
							h.Class("btn-small"),
							g.Attr("data-expand-all", "deployment-group"),
							g.Text("Expand all"),
						)),
					),
					listContent(groups),
				),
			),
		),
	)
}

func listContent(groups []featureGroup) g.Node {
	if len(groups) == 0 {
		return h.P(g.Text("No deployments yet."))
	}

	bodies := make([]g.Node, len(groups))
	for i, fg := range groups {
		bodies[i] = deploymentGroup(fg, i == 0)
	}
	return h.Div(
		h.Class("deployments-list"),
		g.Group(bodies),
	)
}

func deploymentGroup(fg featureGroup, open bool) g.Node {
	detailsAttrs := []g.Node{h.Class("deployment-group")}
	if open {
		detailsAttrs = append(detailsAttrs, h.Open())
	}

	rows := make([]g.Node, 0, len(fg.Targets))
	for _, dep := range fg.Targets {
		rows = append(rows, h.Div(
			h.Class("deployment-row"),
			h.Div(h.Class("dep-version"), versionCell(dep)),
			h.Div(h.Class("dep-status"), statusCell(dep)),
			h.Div(h.Class("dep-target"), targetPills(dep.TargetLabels)),
			h.Div(h.Class("dep-time"), timeWithTitle(dep.createdAt)),
		))
	}

	return h.Details(
		g.Group(detailsAttrs),
		h.Summary(
			h.Class("deployment-group-summary"),
			h.Span(h.Class("dep-feature-toggle")),
			h.A(h.Href("/features/"+fg.FeatureName), g.Text(fg.FeatureName)),
			groupStatusBadge(fg),
			h.Span(h.Class("dep-group-time"), timeWithTitle(fg.LatestTime)),
		),
		h.Div(h.Class("deployment-group-body"), g.Group(rows)),
	)
}

func groupStatusBadge(fg featureGroup) g.Node {
	failed := 0
	pending := 0
	for _, t := range fg.Targets {
		switch t.Status {
		case "FAILED":
			failed++
		case "PENDING":
			pending++
		}
	}
	switch {
	case failed > 0:
		return h.Span(
			h.Class("status-badge status-error"),
			g.Attr("title", failedTitle(failed, len(fg.Targets))),
			g.Textf("%d failed", failed),
		)
	case pending > 0:
		return h.Span(
			h.Class("status-badge status-pending"),
			g.Attr("title", pendingTitle(pending, len(fg.Targets))),
			g.Textf("%d pending", pending),
		)
	}
	return nil
}

func failedTitle(failed, total int) string {
	return fmt.Sprintf("%d of %d targets failed", failed, total)
}

func pendingTitle(pending, total int) string {
	return fmt.Sprintf("%d of %d targets pending", pending, total)
}

func targetPills(labels map[string]string) g.Node {
	if len(labels) == 0 {
		return allEnvironmentsPill()
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pills := make([]g.Node, 0, len(keys))
	for _, k := range keys {
		pills = append(pills, labelPill(k, labels[k]))
	}
	return g.Group(pills)
}

func allEnvironmentsPill() g.Node {
	return h.Span(h.Class("label-filter-tag"), g.Text("All environments"))
}

func labelPill(key, value string) g.Node {
	return h.Span(h.Class("label-filter-tag"), g.Text(key+": "+value))
}

func latestStatusTime(statuses []*deployment.DeploymentStatus) string {
	var latest time.Time
	for _, s := range statuses {
		if s.LastModified.After(latest) {
			latest = s.LastModified
		}
	}

	if latest.IsZero() {
		return ""
	}

	return view.FormatTime(latest)
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
