package auditlog

import (
	"net/http"

	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, *http.Request, layout.Props)

func Handler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, err := audit.ListRecent(r.Context(), 100)
		if err != nil {
			http.Error(w, "Failed to load audit log", http.StatusInternalServerError)
			return
		}

		renderPage(w, r, layout.Props{
			Title:       "Activity Log",
			CurrentPage: components.PageDeployments, // no dedicated page enum yet
			Content:     listPage(entries),
		})
	}
}

func listPage(entries []*audit.Entry) g.Node {
	if len(entries) == 0 {
		return h.Div(h.Class("container"),
			h.Main(h.Class("main-content"),
				h.P(h.Class("text-muted"), g.Text("No activity recorded yet.")),
			),
		)
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

	return h.Div(h.Class("container"),
		h.Main(h.Class("main-content"),
			h.H2(g.Text("Activity Log")),
			h.Table(h.Class("table sortable"), g.Attr("data-sort-key", "auditlog"),
				h.THead(h.Tr(
					h.Th(g.Text("Action")),
					h.Th(g.Text("Resource")),
					h.Th(g.Text("Environment")),
					h.Th(g.Text("Detail")),
					h.Th(g.Text("Actor")),
					h.Th(g.Text("When")),
				)),
				h.TBody(g.Group(rows)),
			),
		),
	)
}
