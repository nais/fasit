package rollouts

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/ui/breadcrumb"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type deployInstructionRow struct {
	Tenant      string
	Environment string
	Status      string
	Created     string
	Logs        []*model.LogLine
}

func DetailHandler(renderPage RenderPage, repo database.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		featureName, version := chi.URLParam(r, "feature"), chi.URLParam(r, "version")
		rollout, err := repo.RolloutByNameAndVersion(r.Context(), featureName, version)
		if err != nil {
			http.Error(w, "Failed to load rollout: "+err.Error(), http.StatusInternalServerError)
			return
		}
		events, _ := repo.RolloutEvents(r.Context(), rollout.ID)

		var instructions []deployInstructionRow
		for _, diID := range rollout.GraphVars.DeployInstructions {
			di, err := repo.DeployInstructionGet(r.Context(), diID)
			if err != nil {
				continue
			}
			tenant, env, err := repo.NamesFromDeployInstruction(r.Context(), diID)
			if err != nil {
				continue
			}
			logs, _ := feature.LogsGet(r.Context(), diID)
			instructions = append(instructions, deployInstructionRow{
				Tenant:      tenant,
				Environment: env,
				Status:      di.Status.String(),
				Created:     view.FormatTime(di.Created),
				Logs:        logs,
			})
		}

		renderPage(w, r, layout.Props{
			Title:       fmt.Sprintf("%s %s", featureName, version),
			CurrentPage: components.PageRollouts,
			Content:     detailPage(rollout, events, instructions),
		})
	}
}

func detailPage(rollout *model.Rollout, events []*model.RolloutEvent, instructions []deployInstructionRow) g.Node {
	var completed g.Node
	if rollout.Completed != nil && !rollout.Completed.IsZero() {
		completed = timeWithTitle(*rollout.Completed)
	} else {
		completed = g.Text("-")
	}

	content := []g.Node{
		h.H1(g.Textf("%s %s", rollout.FeatureName, rollout.Version)),
		h.Table(
			h.Class("table"),
			h.TBody(
				metaRow("Feature", h.A(h.Href("/features/"+rollout.FeatureName), g.Text(rollout.FeatureName))),
				metaRow("Version", g.Text(rollout.Version)),
				metaRow("Status", rolloutStatus(rollout.Status.String())),
				metaRow("Created", timeWithTitle(rollout.Created)),
				metaRow("Completed", completed),
			),
		),
	}

	if len(instructions) > 0 {
		content = append(content, h.H2(g.Text("Deploy Instructions")))
		for _, di := range instructions {
			var logNodes []g.Node
			for _, line := range di.Logs {
				logNodes = append(logNodes, h.Div(
					h.Span(h.Class("text-muted"), g.Text(view.FormatTime(line.Timestamp)+" ")),
					g.Text(line.Message),
				))
			}
			card := h.Div(h.Class("rollout-log"),
				h.Div(h.Class("deployment-group-header"),
					rolloutStatus(di.Status),
					g.Text(" "),
					h.Strong(g.Textf("%s/%s", di.Tenant, di.Environment)),
					h.Span(h.Class("text-muted"), g.Text(" "+di.Created)),
				),
				g.If(len(logNodes) > 0, h.Pre(h.Class("code-block"), g.Group(logNodes))),
			)
			content = append(content, card)
		}
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
						h.Td(timeWithTitle(e.Created)),
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
