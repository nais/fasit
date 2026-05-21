package auditlog

import (
	"net/http"
	"strings"

	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/ui/breadcrumb"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, *http.Request, layout.Props)

func Handler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, err := audit.ListRecent(r.Context(), 200)
		if err != nil {
			http.Error(w, "Failed to load audit log", http.StatusInternalServerError)
			return
		}

		query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
		if query != "" {
			terms := strings.Fields(query)
			filtered := entries[:0]
			for _, e := range entries {
				text := strings.ToLower(string(e.Action) + " " + string(e.ObjectType) + " " + e.ObjectID + " " + e.EnvironmentName + " " + e.TenantName + " " + e.Actor + " " + e.Description)
				if matchesAll(text, terms) {
					filtered = append(filtered, e)
				}
			}
			entries = filtered
		}

		renderPage(w, r, layout.Props{
			Title:       "Activity Log",
			CurrentPage: components.PageDeployments,
			Content:     listPage(entries, query),
		})
	}
}

func matchesAll(text string, terms []string) bool {
	for _, term := range terms {
		if !strings.Contains(text, term) {
			return false
		}
	}
	return true
}

func listPage(entries []*audit.Entry, query string) g.Node {
	return h.Div(
		h.Class("container"),
		h.Main(
			h.Class("main-content"),
			components.Breadcrumbs([]breadcrumb.Crumb{{Label: "Activity Log", URL: "/auditlog"}}),
			h.Div(h.Class("card"), h.Div(h.Class("card-body"),
				h.Div(h.Class("deployments-header"),
					h.Div(h.Class("deployments-toolbar"),
						h.Input(
							h.Type("search"),
							h.Class("table-filter"),
							h.Name("q"),
							h.Placeholder("Filter by action, resource, environment, actor…"),
							g.Attr("aria-label", "Filter activity log"),
							g.Attr("autocomplete", "off"),
							g.Attr("data-url-filter", "q"),
							g.If(query != "", h.Value(query)),
						),
					),
				),
				activityTable(entries),
			)),
		),
	)
}

func activityTable(entries []*audit.Entry) g.Node {
	if len(entries) == 0 {
		return h.P(h.Class("text-muted"), g.Text("No activity recorded."))
	}

	rows := make([]g.Node, 0, len(entries))
	for _, e := range entries {
		env := ""
		if e.TenantName != "" && e.EnvironmentName != "" {
			env = e.TenantName + "/" + e.EnvironmentName
		} else if e.EnvironmentName != "" {
			env = e.EnvironmentName
		}
		rows = append(rows, h.Tr(
			h.Td(g.Text(string(e.Action))),
			h.Td(g.Text(e.ObjectType.Display()+" "+e.ObjectID)),
			h.Td(g.Text(env)),
			h.Td(h.Class("text-muted"), g.Text(e.Description)),
			h.Td(g.Text(e.Actor)),
			h.Td(h.Class("text-muted"), g.Text(view.RelativeTime(e.CreatedAt))),
		))
	}

	return h.Table(h.Class("table sortable"), g.Attr("data-sort-key", "auditlog"),
		h.THead(h.Tr(
			h.Th(g.Text("Action")),
			h.Th(g.Text("Resource")),
			h.Th(g.Text("Environment")),
			h.Th(g.Text("Detail")),
			h.Th(g.Text("Actor")),
			h.Th(g.Text("When")),
		)),
		h.TBody(g.Group(rows)),
	)
}
