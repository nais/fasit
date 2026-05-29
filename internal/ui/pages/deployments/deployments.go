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
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func ListHandler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deps, err := deployment.ListAll(r.Context())
		if err != nil {
			http.Error(w, "Failed to load deployments", http.StatusInternalServerError)
			return
		}

		rows := make([]Summary, 0, len(deps))
		for _, dep := range deps {
			statuses, _ := deployment.ListDeploymentStatuses(r.Context(), dep.ID)
			state, disabledCount := deployment.AggregateState(statuses)
			status := string(state)
			if !dep.Active {
				status = "INACTIVE"
			}
			rows = append(rows, Summary{
				FeatureName:   dep.Feature.Name,
				Chart:         dep.Feature.Chart,
				Version:       dep.Feature.Version,
				Status:        status,
				Target:        deploymentTarget(dep),
				TargetLabels:  deploymentTargetLabels(dep),
				Created:       view.FormatTime(dep.Created),
				Completed:     latestStatusTime(statuses),
				DeploymentID:  dep.ID.String(),
				Active:        dep.Active,
				createdAt:     dep.Created,
				disabledCount: disabledCount,
			})
		}

		sort.Slice(rows, func(i, j int) bool {
			if rows[i].Active != rows[j].Active {
				return rows[i].Active
			}
			return rows[i].createdAt.After(rows[j].createdAt)
		})

		query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
		if query != "" {
			terms := strings.Fields(query)
			filtered := rows[:0]
			for _, row := range rows {
				text := strings.ToLower(row.FeatureName + " " + row.Target + " " + row.Version + " " + row.Status)
				if matchesAll(text, terms) {
					filtered = append(filtered, row)
				}
			}
			rows = filtered
		}

		renderPage(w, r, layout.Props{
			Title:       "Deployments",
			CurrentPage: components.PageDeployments,
			Content:     listPage(rows, query),
			Scripts:     []string{"deployments.js"},
		})
	}
}

// latestPerChartAndTarget returns the most recent deployment for each
// unique (chart URI, target labels) combination.
func latestPerChartAndTarget(deps []*deployment.Deployment) []*deployment.Deployment {
	type key struct {
		chart  string
		target string
	}
	best := make(map[key]*deployment.Deployment)
	for _, dep := range deps {
		k := key{chart: dep.Feature.Chart, target: targetKeyFromLabels(dep.TargetLabels)}
		if existing, ok := best[k]; !ok || dep.Created.After(existing.Created) {
			best[k] = dep
		}
	}
	result := make([]*deployment.Deployment, 0, len(best))
	for _, dep := range best {
		result = append(result, dep)
	}
	return result
}

func matchesAll(text string, terms []string) bool {
	// Normalize "key: value" → "key:value" so either form matches
	normalized := strings.ReplaceAll(text, ": ", ":")
	for _, term := range terms {
		if !strings.Contains(normalized, term) {
			return false
		}
	}
	return true
}

func targetKeyFromLabels(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+labels[k])
	}
	return strings.Join(parts, ",")
}

func listPage(rows []Summary, query string) g.Node {
	return h.Div(
		h.Class("container"),
		components.Breadcrumbs([]breadcrumb.Crumb{breadcrumb.Deployments()}),
		h.Main(
			h.Class("main-content"),
			h.Div(h.Class("card"), h.Div(h.Class("card-body"),
				h.Div(h.Class("deployments-header"),
					h.Div(h.Class("deployments-toolbar"),
						h.Input(
							h.Type("search"),
							h.Class("table-filter"),
							h.Name("q"),
							h.Placeholder("Filter by feature, target, version…"),
							g.Attr("aria-label", "Filter deployments"),
							g.Attr("autocomplete", "off"),
							g.Attr("data-url-filter", "q"),
							g.If(query != "", h.Value(query)),
						),
					),
					h.Button(h.Type("button"), h.Class("btn-small"), g.Attr("popovertarget", "new-deployment"), g.Text("+ New deployment")),
					newDeploymentPopover(),
				),
				deploymentsTable(rows),
			)),
		),
	)
}

func newDeploymentPopover() g.Node {
	return h.Div(g.Attr("popover", ""), h.ID("new-deployment"),
		h.H3(g.Text("New deployment")),
		h.Form(h.Method("POST"), h.Action("/deployments"),
			h.Label(g.Text("Chart")),
			h.Input(h.Type("text"), h.Name("chart"), g.Attr("required", ""), g.Attr("placeholder", "e.g. oci://naiserator")),
			h.Label(g.Text("Version")),
			h.Input(h.Type("text"), h.Name("version"), g.Attr("required", ""), g.Attr("placeholder", "e.g. 2026-05-15-001")),
			h.Label(g.Text("Description (optional)")),
			h.Input(h.Type("text"), h.Name("description"), g.Attr("placeholder", "e.g. Rollback to stable")),
			h.Label(g.Text("Target labels")),
			h.Textarea(h.Name("target_labels_raw"), h.ID("target-labels-input"), g.Attr("rows", "4"), g.Attr("placeholder", "{\n  \"kind\": \"tenant\",\n  \"tenant\": \"nav\"\n}")),
			h.Div(h.Class("form-hint-row"),
				h.A(h.Href("/labels"), g.Attr("target", "_blank"), h.Class("form-hint"), g.Text("Browse labels")),
				h.Button(h.Type("button"), h.Class("btn-small btn-outline"), h.ID("preview-targets-btn"), g.Text("Preview targets")),
			),
			h.Div(h.ID("preview-targets-result"), h.Class("preview-targets-result")),
			h.Div(h.Class("popover-actions"),
				h.Button(h.Type("submit"), h.ID("deploy-submit-btn"), g.Text("Deploy")),
				h.Button(h.Type("button"), g.Attr("popovertarget", "new-deployment"), g.Attr("popovertargetaction", "hide"), g.Text("Cancel")),
			),
		),
	)
}

func deploymentsTable(rows []Summary) g.Node {
	if len(rows) == 0 {
		return h.P(h.Class("text-muted"), g.Text("No deployments."))
	}

	tableRows := make([]g.Node, 0, len(rows))
	for _, dep := range rows {
		tableRows = append(tableRows, h.Tr(
			h.Td(h.A(h.Href("/features/"+dep.FeatureName), g.Text(dep.FeatureName))),
			h.Td(targetPills(dep.TargetLabels)),
			h.Td(h.A(h.Href("/deployments/"+dep.DeploymentID), g.Text(dep.Version))),
			h.Td(statusCell(dep)),
			h.Td(g.Attr("data-sort-value", dep.createdAt.Format(time.RFC3339)), h.Title(view.FormatTime(dep.createdAt)), g.Text(view.RelativeTime(dep.createdAt))),
		))
	}

	return h.Table(
		h.Class("table sortable"),
		h.ID("deployments-table"),
		g.Attr("data-sort-key", "deployments"),
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
