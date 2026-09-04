package assignments

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/model"
	"github.com/nais/fasit/internal/reconciler"
	"github.com/nais/fasit/internal/ui/breadcrumb"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func ListHandler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fas, err := featureassignment.ListAll(r.Context())
		if err != nil {
			http.Error(w, "Failed to load assignments", http.StatusInternalServerError)
			return
		}

		statusesByID, err := reconciler.AllReconcileStatuses(r.Context())
		if err != nil {
			http.Error(w, "Failed to load assignment statuses", http.StatusInternalServerError)
			return
		}

		assignmentIDs := make([]uuid.UUID, 0, len(fas))
		for _, fa := range fas {
			assignmentIDs = append(assignmentIDs, fa.ID)
		}
		creators, err := audit.AssignmentCreators(r.Context(), assignmentIDs)
		if err != nil {
			http.Error(w, "Failed to load assignment creators", http.StatusInternalServerError)
			return
		}

		rows := make([]Summary, 0, len(fas))
		for _, fa := range fas {
			statuses := statusesByID[fa.ID]

			states := make(model.FeatureReconcileStatusStates, len(statuses))
			for i, s := range statuses {
				states[i] = model.FeatureReconcileStatusState(strings.ToUpper(string(s.State)))
			}
			state, disabledCount := states.Aggregate()
			status := string(state)
			if !fa.Active {
				status = "INACTIVE"
			}
			rows = append(rows, Summary{
				FeatureName:         fa.Feature.Name,
				Chart:               fa.Feature.Chart,
				Version:             fa.Feature.Version,
				Status:              status,
				Target:              assignmentTarget(fa),
				TargetLabels:        assignmentTargetLabels(fa),
				Creator:             creators[fa.ID],
				Created:             view.FormatTime(fa.Created),
				Completed:           latestStatusTime(statuses),
				FeatureAssignmentID: fa.ID.String(),
				Active:              fa.Active,
				createdAt:           fa.Created,
				disabledCount:       disabledCount,
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
				text := strings.ToLower(row.FeatureName + " " + row.Target + " " + row.Version + " " + row.Status + " " + row.Creator)
				if matchesAll(text, terms) {
					filtered = append(filtered, row)
				}
			}
			rows = filtered
		}

		renderPage(w, r, layout.Props{
			Title:       "Assignments",
			CurrentPage: components.PageAssignments,
			Content:     listPage(rows, query),
			Scripts:     []string{"assignments.js"},
		})
	}
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

func listPage(rows []Summary, query string) g.Node {
	return h.Div(
		h.Class("container"),
		components.Breadcrumbs([]breadcrumb.Crumb{breadcrumb.Assignments()}),
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
							h.Placeholder("Filter by feature, target, version…"),
							g.Attr("aria-label", "Filter assignments"),
							g.Attr("autocomplete", "off"),
							g.Attr("data-url-filter", "q"),
							g.If(query != "", h.Value(query)),
						),
					),
					h.Button(h.Type("button"), h.Class("btn-small"), g.Attr("popovertarget", "new-assignment"), g.Text("+ New assignment")),
					newAssignmentPopover(),
				),
				assignmentsTable(rows),
			)),
		),
	)
}

func newAssignmentPopover() g.Node {
	return components.Popover(
		"new-assignment", "", "New assignment",
		h.Form(
			h.Method("POST"), h.Action("/assignments"),
			h.Label(g.Text("Chart")),
			h.Input(h.Type("text"), h.Name("chart"), g.Attr("required", ""), g.Attr("placeholder", "e.g. oci://naiserator")),
			h.Label(g.Text("Version")),
			h.Input(h.Type("text"), h.Name("version"), g.Attr("required", ""), g.Attr("placeholder", "e.g. 2026-05-15-001")),
			h.Label(g.Text("Description (optional)")),
			h.Input(h.Type("text"), h.Name("description"), g.Attr("placeholder", "e.g. Rollback to stable")),
			h.Label(g.Text("Target labels")),
			h.Textarea(h.Name("target_labels_raw"), h.ID("target-labels-input"), g.Attr("rows", "4"), g.Attr("placeholder", "{\n  \"kind\": \"tenant\",\n  \"tenant\": \"nav\"\n}")),
			h.Div(
				h.Class("form-hint-row"),
				h.A(h.Href("/labels"), g.Attr("target", "_blank"), h.Class("form-hint"), g.Text("Browse labels")),
				h.Button(h.Type("button"), h.Class("btn-small btn-outline"), h.ID("preview-targets-btn"), g.Text("Preview targets")),
			),
			h.Div(h.ID("preview-targets-result"), h.Class("preview-targets-result")),
			components.PopoverActions(
				h.Button(h.Type("submit"), h.ID("deploy-submit-btn"), g.Text("Deploy")),
			),
		),
	)
}

func assignmentsTable(rows []Summary) g.Node {
	if len(rows) == 0 {
		return h.P(h.Class("text-muted"), g.Text("No assignments."))
	}

	tableRows := make([]g.Node, 0, len(rows))
	for _, dep := range rows {
		rowClass := ""
		if !view.IsWorkflowActor(dep.Creator) {
			rowClass = "assignment-non-workflow"
		}
		tableRows = append(tableRows, h.Tr(
			g.If(rowClass != "", h.Class(rowClass)),
			h.Td(h.A(h.Href("/features/"+dep.FeatureName), g.Text(dep.FeatureName))),
			h.Td(targetPills(dep.TargetLabels)),
			h.Td(h.A(h.Href("/features/"+dep.FeatureName+"/assignments/"+dep.FeatureAssignmentID), g.Text(dep.Version))),
			h.Td(statusCell(dep)),
			h.Td(view.AssignmentCreatorNode(dep.Creator)),
			view.TimeCell(dep.createdAt),
		))
	}

	return h.Table(
		h.Class("table sortable"),
		h.ID("assignments-table"),
		g.Attr("data-sort-key", "assignments"),
		h.THead(h.Tr(
			h.Th(g.Text("Feature")),
			h.Th(g.Text("Target"), g.Attr("data-no-sort", "")),
			h.Th(g.Text("Version")),
			h.Th(g.Text("Status")),
			h.Th(g.Text("Created by")),
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

func latestStatusTime(statuses []*reconciler.FeatureReconcileStatus) string {
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

func assignmentTarget(dep *featureassignment.FeatureAssignment) string {
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

func assignmentTargetLabels(dep *featureassignment.FeatureAssignment) map[string]string {
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
