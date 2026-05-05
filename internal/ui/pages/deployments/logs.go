package deployments

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/ui/breadcrumb"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func LogsHandler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			http.Error(w, "Failed to parse deployment ID", http.StatusInternalServerError)
			return
		}

		envID, err := uuid.Parse(chi.URLParam(r, "envID"))
		if err != nil {
			http.Error(w, "Failed to parse environment ID", http.StatusInternalServerError)
			return
		}

		dep, err := deployment.GetDeployment(r.Context(), id)
		if err != nil {
			http.Error(w, "Failed to load deployment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		log, err := deployment.GetDeploymentStatusLog(r.Context(), id, envID)
		if err != nil {
			http.Error(w, "Failed to load logs: "+err.Error(), http.StatusInternalServerError)
			return
		}

		title := fmt.Sprintf("Deployment Logs: %s v%s", dep.Feature.Name, dep.Feature.Version)
		renderPage(w, r, layout.Props{
			Title:       title,
			CurrentPage: components.PageDeployments,
			Content:     logsPage(dep, log),
		})
	}
}

func logsPage(dep *deployment.Deployment, log *model.RolloutLog) g.Node {
	var logContent []g.Node

	if log == nil || len(log.Lines) == 0 {
		logContent = append(logContent, h.P(g.Text("No logs available.")))
	} else {
		var sb strings.Builder
		for i, line := range log.Lines {
			if i > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(view.FormatTime(line.Timestamp))
			sb.WriteString(" ")
			sb.WriteString(line.Message)
		}
		logContent = append(logContent,
			h.P(h.Class("text-muted"), g.Textf("Environment: %s", log.Environment)),
			h.Pre(h.Class("code-block"), g.Text(sb.String())),
		)
	}

	content := []g.Node{
		h.H1(g.Textf("Deployment Logs: %s v%s", dep.Feature.Name, dep.Feature.Version)),
	}
	content = append(content, logContent...)

	return h.Div(
		h.Class("container"),
		h.Main(
			h.Class("main-content"),
			components.Breadcrumbs([]breadcrumb.Crumb{
				breadcrumb.Deployments(),
				breadcrumb.Deployment(dep.ID.String(), dep.Feature.Name, dep.Feature.Version),
				{Label: "Logs"},
			}),
			h.Div(h.Class("card"), h.Div(h.Class("card-body"), g.Group(content))),
		),
	)
}
