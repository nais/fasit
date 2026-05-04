package deployments

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/ui/breadcrumb"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
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
				status, disabledCount := aggregateDeploymentStatus(statuses)
				items = append(items, Summary{
					FeatureName:   dep.Feature.Name,
					Version:       dep.Feature.Version,
					Status:        status,
					Target:        deploymentTarget(dep),
					Created:       formatTime(dep.Created),
					Completed:     latestStatusTime(statuses),
					DeploymentID:  dep.ID.String(),
					createdAt:     dep.Created,
					disabledCount: disabledCount,
				})
			}
		}

		groups := groupDeployments(items)

		renderPage(w, layout.Props{
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

	nodes := make([]g.Node, len(groups))
	for i, fg := range groups {
		isFirst := i == 0
		nodes[i] = deploymentGroup(fg, isFirst)
	}
	return h.Div(h.Class("deployment-groups"), g.Group(nodes))
}

func deploymentGroup(fg featureGroup, open bool) g.Node {
	detailsAttrs := []g.Node{
		h.Class("deployment-group"),
	}
	if open {
		detailsAttrs = append(detailsAttrs, g.Attr("open"))
	}

	rows := make([]g.Node, len(fg.Targets))
	for i, dep := range fg.Targets {
		rows[i] = h.Div(
			h.Class("deployment-row"),
			h.Span(h.Class("dep-target"), g.Text(dep.Target)),
			h.Span(h.Class("dep-version"), versionCell(dep)),
			h.Span(h.Class("dep-status"), statusCell(dep)),
			h.Span(h.Class("dep-time"), g.Text(dep.Created)),
		)
	}

	return h.Details(
		g.Group(detailsAttrs),
		h.Summary(
			h.Class("deployment-group-summary"),
			h.A(h.Href("/ui/features/"+fg.FeatureName), g.Text(fg.FeatureName)),
			h.Span(h.Class("dep-group-time"), g.Text(formatTime(fg.LatestTime))),
		),
		h.Div(h.Class("deployment-group-body"), g.Group(rows)),
	)
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

	return formatTime(latest)
}

func deploymentTarget(dep *deployment.Deployment) string {
	if dep.CI {
		return "CI"
	}

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
