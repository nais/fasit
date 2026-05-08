package deployments

import (
	"fmt"
	"maps"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/deployment/deploymentsql"
	envpkg "github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/ui/breadcrumb"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type deploymentStatusRow struct {
	TenantName, EnvironmentName, EnvironmentID, State, Message string
	LastModified                                               time.Time
}

type matchingDeployment struct {
	ID, Version string
	Created     time.Time
}

func DetailHandler(renderPage RenderPage, repo database.Repo) http.HandlerFunc {
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

			releaseVersion := ""
			if releases, err := repo.ReleaseStatusesGet(r.Context(), status.EnvironmentID); err == nil {
				for _, release := range releases {
					if release.Name == dep.Feature.Name {
						releaseVersion = release.Version
						break
					}
				}
			}

			state, lastMod := view.EffectiveDeploymentStatus(r.Context(), repo, status.EnvironmentID, dep.Feature.Name, status.State.String(), status.LastModified, dep.Feature.Version, releaseVersion)
			rows = append(rows, deploymentStatusRow{
				TenantName:      tenant.Name,
				EnvironmentName: env.Name,
				EnvironmentID:   status.EnvironmentID.String(),
				State:           state,
				Message:         status.Message,
				LastModified:    lastMod,
			})
		}

		allDeployInstructions, err := deployment.ListDeployInstructionsByDeploymentID(r.Context(), id)
		if err != nil {
			http.Error(w, "Failed to load deploy instructions: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Deduplicate to latest instruction per environment (query is ordered by created DESC).
		seen := make(map[string]bool)
		var deployInstructions []deploymentsql.ListDeployInstructionsByDeploymentIDRow
		for _, di := range allDeployInstructions {
			envID := di.DeployInstruction.EnvironmentID.String()
			if !seen[envID] {
				seen[envID] = true
				deployInstructions = append(deployInstructions, di)
			}
		}

		allByFeature, _ := deployment.ListDeploymentsByFeature(r.Context(), dep.Feature.Name)
		var matching []matchingDeployment
		for _, d := range allByFeature {
			if !maps.Equal(map[string]string(d.TargetLabels), map[string]string(dep.TargetLabels)) {
				continue
			}
			matching = append(matching, matchingDeployment{
				ID:      d.ID.String(),
				Version: d.Feature.Version,
				Created: d.Created,
			})
		}

		renderPage(w, r, layout.Props{Title: fmt.Sprintf("Deployment %s %s", dep.Feature.Name, dep.Feature.Version), CurrentPage: components.PageDeployments, Content: detailPage(dep, rows, deployInstructions, matching)})
	}
}

func detailPage(d *deployment.Deployment, statuses []deploymentStatusRow, deployInstructions []deploymentsql.ListDeployInstructionsByDeploymentIDRow, matching []matchingDeployment) g.Node {
	meta := []g.Node{
		metaRow("Feature", h.A(h.Href("/features/"+d.Feature.Name), g.Text(d.Feature.Name))),
		metaRow("Version", g.Text(d.Feature.Version)),
		metaRow("Target", targetPills(deploymentTargetLabels(d))),
		metaRow("Created", timeWithTitle(d.Created)),
	}

	if d.Description != nil && *d.Description != "" {
		meta = append(meta, metaRow("Description", g.Text(*d.Description)))
	}

	meta = append(meta, metaRow("Actions", h.Div(
		h.Button(h.Type("button"), h.Class("btn-small"), g.Attr("popovertarget", "delete-deployment"), g.Text("Delete")),
		h.Div(g.Attr("popover", ""), h.ID("delete-deployment"),
			h.H3(g.Text("Delete deployment")),
			h.P(g.Textf("Delete deployment %s %s?", d.Feature.Name, d.Feature.Version)),
			h.Form(h.Method("POST"), h.Action("/deployments/"+d.ID.String()+"/delete"),
				h.Div(h.Class("popover-actions"), h.Button(h.Type("submit"), g.Text("Delete")), h.Button(h.Type("button"), g.Attr("popovertarget", "delete-deployment"), g.Attr("popovertargetaction", "hide"), g.Text("Cancel"))),
			),
		),
	)))

	content := []g.Node{
		h.H1(g.Textf("Deployment: %s %s", d.Feature.Name, d.Feature.Version)),
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
				h.Th(g.Text("Logs")),
			)),
			h.TBody(g.Group(g.Map(statuses, func(s deploymentStatusRow) g.Node {
				return h.Tr(
					h.Td(h.A(h.Href("/tenants/"+s.TenantName), g.Text(s.TenantName))),
					h.Td(h.A(h.Href("/tenants/"+s.TenantName+"/envs/"+s.EnvironmentName+"/"+d.Feature.Name), g.Text(s.EnvironmentName))),
					h.Td(rolloutStatus(s.State)),
					h.Td(g.Text(s.Message)),
					h.Td(timeWithTitle(s.LastModified)),
					h.Td(h.A(h.Href("/deployments/"+d.ID.String()+"/logs/"+s.EnvironmentID), g.Attr("title", "View logs"), g.Text("📋"))),
				)
			}))),
		))
	}

	content = append(content,
		h.H2(g.Text("All deployments matching target")),
		h.Table(
			h.Class("table"),
			h.THead(h.Tr(
				h.Th(g.Text("Version")),
				h.Th(g.Text("Created")),
			)),
			h.TBody(g.Group(g.Map(matching, func(m matchingDeployment) g.Node {
				return h.Tr(
					h.Td(h.A(h.Href("/deployments/"+m.ID), g.Text(m.Version))),
					h.Td(timeWithTitle(m.Created)),
				)
			}))),
		),
		h.Button(h.Type("button"), h.Class("btn-small"), g.Attr("popovertarget", "delete-all-deployments"), g.Text("Delete all deployments")),
		g.Textf(" (%d)", len(matching)),
		h.Div(g.Attr("popover", ""), h.ID("delete-all-deployments"),
			h.H3(g.Text("Delete all deployments")),
			h.P(g.Textf("Delete all %d deployments for %s?", len(matching), d.Feature.Name)),
			h.Form(h.Method("POST"), h.Action("/deployments/"+d.ID.String()+"/delete-matching"),
				h.Div(h.Class("popover-actions"), h.Button(h.Type("submit"), g.Text("Delete all")), h.Button(h.Type("button"), g.Attr("popovertarget", "delete-all-deployments"), g.Attr("popovertargetaction", "hide"), g.Text("Cancel"))),
			),
		),
	)

	var naidInstructionsContent g.Node
	if len(deployInstructions) == 0 {
		naidInstructionsContent = h.P(g.Text("No NAISD instructions."))
	} else {
		naidInstructionsContent = h.Table(
			h.Class("table sortable"),
			h.THead(h.Tr(
				h.Th(g.Text("Environment")),
				h.Th(g.Text("Status")),
				h.Th(g.Text("Created")),
				h.Th(g.Text("Last Modified")),
				h.Th(g.Text("Logs")),
			)),
			h.TBody(g.Group(g.Map(deployInstructions, func(di deploymentsql.ListDeployInstructionsByDeploymentIDRow) g.Node {
				return h.Tr(
					h.Td(h.A(h.Href("/tenants/"+di.TenantName+"/envs/"+di.EnvironmentName), g.Textf("%s / %s", di.TenantName, di.EnvironmentName))),
					h.Td(rolloutStatus(strings.ToUpper(di.DeployInstruction.Status))),
					h.Td(timeWithTitle(di.DeployInstruction.Created.Time)),
					h.Td(timeWithTitle(di.DeployInstruction.LastModified.Time)),
					h.Td(h.A(h.Href("/deployments/"+d.ID.String()+"/logs/"+di.DeployInstruction.EnvironmentID.String()), g.Attr("title", "View logs"), g.Text("📋"))),
				)
			}))),
		)
	}
	content = append(content,
		h.H2(g.Text("NAISD Instructions")),
		h.Details(
			h.Summary(g.Text("Show")),
			naidInstructionsContent,
		),
	)

	return h.Div(
		h.Class("container"),
		h.Main(
			h.Class("main-content"),
			components.Breadcrumbs([]breadcrumb.Crumb{breadcrumb.Deployments(), breadcrumb.Deployment(d.ID.String(), d.Feature.Name, d.Feature.Version)}),
			h.Div(h.Class("card"), h.Div(h.Class("card-body"), g.Group(content))),
		),
	)
}
