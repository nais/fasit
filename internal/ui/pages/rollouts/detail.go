package rollouts

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/ui/breadcrumb"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func DetailHandler(renderPage RenderPage, repo database.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		featureName, version := chi.URLParam(r, "feature"), chi.URLParam(r, "version")
		rollout, err := repo.RolloutByNameAndVersion(r.Context(), featureName, version)
		if err != nil {
			http.Error(w, "Failed to load rollout: "+err.Error(), http.StatusInternalServerError)
			return
		}
		events, _ := repo.RolloutEvents(r.Context(), rollout.ID)
		renderPage(w, layout.Props{
			Title:          fmt.Sprintf("%s v%s", featureName, version),
			CurrentPage: components.PageRollouts,
			Content:        detailPage(rollout, events),
		})
	}
}

func detailPage(rollout *model.Rollout, events []*model.RolloutEvent) g.Node {
	completed := formatTimePtr(rollout.Completed)
	if completed == "" {
		completed = "-"
	}

	content := []g.Node{
		h.H1(g.Textf("%s v%s", rollout.FeatureName, rollout.Version)),
		h.Table(
			h.Class("table"),
			h.TBody(
				metaRow("Feature", h.A(h.Href("/ui/features/"+rollout.FeatureName), g.Text(rollout.FeatureName))),
				metaRow("Version", g.Text(rollout.Version)),
				metaRow("Status", rolloutStatus(rollout.Status.String())),
				metaRow("Created", g.Text(formatTime(rollout.Created))),
				metaRow("Completed", g.Text(completed)),
			),
		),
	}

	if len(events) > 0 {
		content = append(
			content,
			h.H2(g.Text("Events")),
			h.Table(
				h.Class("table"),
				h.THead(h.Tr(h.Th(g.Text("Time")), h.Th(g.Text("Message")))),
				h.TBody(g.Group(g.Map(events, func(e *model.RolloutEvent) g.Node {
					cls := ""
					if e.Failure {
						cls = "status-error"
					}

					return h.Tr(
						h.Td(g.Text(formatTime(e.Created))),
						h.Td(h.Class(cls), g.Text(e.Message)),
					)
				}))),
			),
		)
	}

	return h.Div(
		h.Class("container"),
		h.Main(
			h.Class("main-content"),
			components.Breadcrumbs([]breadcrumb.Crumb{breadcrumb.Rollouts(), breadcrumb.Rollout(rollout.FeatureName, rollout.Version)}),
			h.Div(h.Class("card"), h.Div(h.Class("card-body"), g.Group(content))),
		),
	)
}

func metaRow(label string, value g.Node) g.Node {
	return h.Tr(h.Td(h.Class("th-like"), g.Text(label)), h.Td(value))
}
