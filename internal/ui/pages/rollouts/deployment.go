package rollouts

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/deployment"
	envpkg "github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/ui"
	"github.com/nais/fasit/internal/ui/breadcrumb"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type deploymentStatusRow struct{ TenantName, EnvironmentName, State, Message, LastModified string }

func DeploymentHandler(renderPage RenderPage, repo database.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			http.Error(w, "Failed to load deployment", http.StatusInternalServerError)
			return
		}

		dep, err := deployment.GetDeployment(r.Context(), id)
		if err != nil {
			http.Error(w, "Failed to load deployment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		statuses, _ := deployment.ListDeploymentStatuses(r.Context(), id)

		rows := make([]deploymentStatusRow, 0, len(statuses))
		for _, status := range statuses {
			env, err := repo.EnvironmentGet(r.Context(), status.EnvironmentID)
			if err != nil {
				continue
			}

			tenant, err := envpkg.GetTenant(r.Context(), env.TenantID)
			if err != nil {
				continue
			}

			rows = append(rows, deploymentStatusRow{
				TenantName:      tenant.Name,
				EnvironmentName: env.Name,
				State:           status.State.String(),
				Message:         status.Message,
				LastModified:    formatTime(status.LastModified),
			})
		}

		renderPage(w, layout.Props{Title: fmt.Sprintf("Deployment %s v%s", dep.Feature.Name, dep.Feature.Version), CurrentSection: "rollouts", Content: deploymentPage(dep, rows)})
	}
}

func deploymentPage(d *deployment.Deployment, statuses []deploymentStatusRow) g.Node {
	meta := []g.Node{
		metaRow("Feature", h.A(h.Href(ui.BasePath+"/features/"+d.Feature.Name+"/"), g.Text(d.Feature.Name))),
		metaRow("Version", g.Text(d.Feature.Version)),
		metaRow("Target", g.Text(deploymentTarget(d))),
		metaRow("Created", g.Text(formatTime(d.Created))),
	}

	if d.Description != nil && *d.Description != "" {
		meta = append(meta, metaRow("Description", g.Text(*d.Description)))
	}

	content := []g.Node{
		h.H1(g.Textf("Deployment: %s v%s", d.Feature.Name, d.Feature.Version)),
		h.Table(h.Class("table"), h.TBody(g.Group(meta))),
		h.H2(g.Text("Environment Statuses")),
	}

	if len(statuses) == 0 {
		content = append(content, h.P(g.Text("No statuses.")))
	} else {
		content = append(content, h.Table(
			h.Class("table sortable"),
			h.THead(h.Tr(
				h.Th(g.Text("Tenant")),
				h.Th(g.Text("Environment")),
				h.Th(g.Text("State")),
				h.Th(g.Text("Message")),
				h.Th(g.Text("Last Modified")),
			)),
			h.TBody(g.Group(g.Map(statuses, func(s deploymentStatusRow) g.Node {
				return h.Tr(
					h.Td(h.A(h.Href(ui.BasePath+"/tenants/"+s.TenantName+"/"), g.Text(s.TenantName))),
					h.Td(h.A(h.Href(ui.BasePath+"/tenants/"+s.TenantName+"/envs/"+s.EnvironmentName+"/"+d.Feature.Name+"/"), g.Text(s.EnvironmentName))),
					h.Td(rolloutStatus(s.State)),
					h.Td(g.Text(s.Message)),
					h.Td(g.Text(s.LastModified)),
				)
			}))),
		))
	}

	return h.Div(
		h.Class("container"),
		h.Main(
			h.Class("main-content"),
			components.Breadcrumbs([]breadcrumb.Crumb{breadcrumb.Rollouts(), breadcrumb.Deployment(d.ID.String(), d.Feature.Name, d.Feature.Version)}),
			h.Div(h.Class("card"), h.Div(h.Class("card-body"), g.Group(content))),
		),
	)
}
