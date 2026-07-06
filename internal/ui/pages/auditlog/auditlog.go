package auditlog

import (
	"net/http"
	"strings"

	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/ui/auditview"
	"github.com/nais/fasit/internal/ui/breadcrumb"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, *http.Request, layout.Props)

func Handler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		var entries []*audit.Entry
		var err error
		if query == "" {
			entries, err = audit.ListRecent(r.Context(), 200)
		} else {
			entries, err = audit.SearchRecent(r.Context(), strings.Fields(query), 200)
		}
		if err != nil {
			http.Error(w, "Failed to load audit log", http.StatusInternalServerError)
			return
		}

		renderPage(w, r, layout.Props{
			Title:       "Audit log",
			CurrentPage: components.PageAssignments,
			Content:     listPage(entries, query),
		})
	}
}

func listPage(entries []*audit.Entry, query string) g.Node {
	return h.Div(
		h.Class("container"),
		components.Breadcrumbs([]breadcrumb.Crumb{{Label: "Audit log", URL: "/auditlog"}}),
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
	return auditview.ActivityTable(entries, "auditlog")
}
