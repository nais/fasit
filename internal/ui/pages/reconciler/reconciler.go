package reconcilerpage

import (
	"net/http"

	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, *http.Request, layout.Props)

func Handler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, r, layout.Props{
			Title:       "Reconciler",
			CurrentPage: components.PageReconciler,
			Content:     triggerPage(),
			Scripts:     []string{"reconciler.js"},
		})
	}
}

func triggerPage() g.Node {
	return h.Main(
		h.Class("main-content"),
		h.Div(
			h.Class("reconciler-header"),
			h.H1(g.Text("Reconciler")),
			h.Button(h.Type("button"), h.Class("btn"), h.ID("reconcile-btn"), g.Text("Run reconcile")),
		),
		h.P(h.Class("text-muted"), h.ID("reconcile-status"), g.Text("Run a dry-run reconcile to see what would be deployed.")),
		h.Div(h.ID("reconcile-summary")),
		h.Div(
			h.Class("card"), h.ID("reconcile-table-card"), g.Attr("style", "display:none"),
			h.Div(
				h.Class("card-body"),
				h.Table(
					h.Class("table"), h.ID("reconcile-table"),
					h.THead(h.Tr(
						h.Th(g.Text("Action")),
						h.Th(g.Text("Tenant")),
						h.Th(g.Text("Environment")),
						h.Th(g.Text("Feature")),
						h.Th(g.Text("Version")),
						h.Th(g.Text("Message")),
					)),
					h.TBody(h.ID("reconcile-tbody")),
				),
			),
		),
	)
}
