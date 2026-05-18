package deployments

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/deployment"
	envpkg "github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/graph/model"
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

type versionHistoryEntry struct {
	ID, Version, Description string
	GHRef                    *model.GHRef
	Created                  time.Time
	Active                   bool
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

		allByFeature, _ := deployment.ListDeploymentsByFeature(r.Context(), dep.Feature.Name)
		var history []versionHistoryEntry
		for _, d := range allByFeature {
			if !maps.Equal(map[string]string(d.TargetLabels), map[string]string(dep.TargetLabels)) {
				continue
			}
			desc := ""
			if d.Description != nil {
				desc = *d.Description
			}
			history = append(history, versionHistoryEntry{
				ID:          d.ID.String(),
				Version:     d.Feature.Version,
				Description: desc,
				GHRef:       parseGHRef(d.GHRef),
				Created:     d.Created,
				Active:      d.Active,
			})
		}

		sort.SliceStable(history, func(i, j int) bool {
			if history[i].Active != history[j].Active {
				return history[i].Active
			}
			return history[i].Created.After(history[j].Created)
		})

		// Use the active deployment for meta display, fall back to the URL'd one
		current := dep
		for _, d := range allByFeature {
			if d.Active && maps.Equal(map[string]string(d.TargetLabels), map[string]string(dep.TargetLabels)) {
				current = d
				break
			}
		}

		renderPage(w, r, layout.Props{
			Title:       fmt.Sprintf("Deployment %s %s", current.Feature.Name, current.Feature.Version),
			CurrentPage: components.PageDeployments,
			Content:     detailPage(current, rows, history),
		})
	}
}

func detailPage(d *deployment.Deployment, statuses []deploymentStatusRow, history []versionHistoryEntry) g.Node {
	var statusBadge g.Node
	if d.Active {
		statusBadge = h.Span(h.Class("status-badge status-success"), g.Text("active"))
	} else {
		statusBadge = h.Span(h.Class("status-badge status-disabled"), g.Text("inactive"))
	}

	meta := []g.Node{
		metaRow("Target", targetPills(deploymentTargetLabels(d))),
		metaRow("Status", statusBadge),
		metaRow("Version", g.Text(d.Feature.Version)),
		metaRow("Updated", timeWithTitle(d.Created)),
	}
	if ref := parseGHRef(d.GHRef); ref != nil {
		meta = append(meta, metaRow("Commit", ghRefLink(ref)))
	}
	if d.Description != nil && *d.Description != "" {
		meta = append(meta, metaRow("Description", g.Text(*d.Description)))
	}

	content := []g.Node{
		h.Div(h.Class("deploy-detail-topbar"),
			h.H1(h.A(h.Href("/features/"+d.Feature.Name), g.Text(d.Feature.Name))),
			h.Details(h.Class("env-actions"),
				h.Summary(h.Class("env-actions-toggle"), g.Attr("title", "Actions"), g.Text("\u22ee")),
				h.Div(h.Class("env-actions-menu"),
					h.Button(h.Type("button"), h.Class("env-actions-danger"), g.Attr("popovertarget", "delete-deployment"), g.Text("Delete all history")),
				),
				h.Div(g.Attr("popover", ""), h.ID("delete-deployment"),
					h.H3(g.Text("Delete all deployments")),
					h.P(g.Textf("This will permanently delete all %d versions for %s with this target. This cannot be undone.", len(history), d.Feature.Name)),
					h.Form(h.Method("POST"), h.Action("/deployments/"+d.ID.String()+"/delete-matching"),
						h.Input(h.Type("hidden"), h.Name("redirect"), h.Value("/features/"+d.Feature.Name)),
						h.Div(h.Class("popover-actions"),
							h.Button(h.Type("submit"), g.Text("Delete all")),
							h.Button(h.Type("button"), g.Attr("popovertarget", "delete-deployment"), g.Attr("popovertargetaction", "hide"), g.Text("Cancel")),
						),
					),
				),
			),
		),
		h.Table(h.Class("table meta-table table-compact"), h.TBody(g.Group(meta))),
	}

	// Environment statuses
	if len(statuses) > 0 {
		content = append(content,
			h.H2(g.Text("Environment statuses")),
			h.Table(
				h.Class("table sortable"),
				g.Attr("data-sort-key", "deployment-detail-envs"),
				h.THead(h.Tr(
					h.Th(g.Text("Tenant")),
					h.Th(g.Text("Environment")),
					h.Th(g.Text("State")),
					h.Th(g.Text("Message")),
					h.Th(g.Text("Last Modified")),
					h.Th(g.Text("")),
				)),
				h.TBody(g.Group(g.Map(statuses, func(s deploymentStatusRow) g.Node {
					return h.Tr(
						h.Td(h.A(h.Href("/tenants/"+s.TenantName), g.Text(s.TenantName))),
						h.Td(h.A(h.Href("/tenants/"+s.TenantName+"/envs/"+s.EnvironmentName+"/"+d.Feature.Name), g.Text(s.EnvironmentName))),
						h.Td(rolloutStatus(s.State)),
						h.Td(g.Text(s.Message)),
						h.Td(timeWithTitle(s.LastModified)),
						h.Td(h.A(h.Href("/deployments/"+d.ID.String()+"/logs/"+s.EnvironmentID), h.Class("env-logs-link"), g.Attr("title", "View logs"), g.Text("\U0001fab5 logs"))),
					)
				}))),
			),
		)
	}

	// Versions
	content = append(content,
		h.H2(g.Text("Versions")),
		versionHistoryTable(history, d),
	)

	return h.Div(
		h.Class("container"),
		h.Main(
			h.Class("main-content"),
			components.Breadcrumbs([]breadcrumb.Crumb{breadcrumb.Deployments(), breadcrumb.Deployment(d.ID.String(), d.Feature.Name, deploymentTarget(d))}),
			h.Div(h.Class("card"), h.Div(h.Class("card-body"), g.Group(content))),
		),
	)
}

func versionHistoryTable(history []versionHistoryEntry, d *deployment.Deployment) g.Node {
	rows := g.Map(history, func(m versionHistoryEntry) g.Node {
		versionNode := g.Text(m.Version)
		var badges []g.Node
		if m.Active {
			badges = append(badges, h.Span(h.Class("status-badge status-success"), g.Text("active")))
		}

		var descNode g.Node
		if m.Description != "" {
			descNode = g.Text(m.Description)
		}

		var refNode g.Node
		if m.GHRef != nil {
			refNode = ghRefLink(m.GHRef)
		}

		var actionNode g.Node
		if m.Active {
			actionNode = h.Form(h.Method("POST"), h.Action("/deployments/"+m.ID+"/deactivate"),
				h.Input(h.Type("hidden"), h.Name("redirect"), h.Value("/deployments/"+m.ID)),
				h.Button(h.Type("submit"), h.Class("btn-small btn-danger"), g.Text("Deactivate")),
			)
		} else {
			actionNode = h.Form(h.Method("POST"), h.Action("/deployments/"+m.ID+"/activate-version"),
				h.Input(h.Type("hidden"), h.Name("redirect"), h.Value("/deployments/"+m.ID)),
				h.Button(h.Type("submit"), h.Class("btn-small"), g.Text("Activate")),
			)
		}

		return h.Tr(
			h.Td(versionNode, g.Text(" "), g.Group(badges)),
			h.Td(refNode),
			h.Td(descNode),
			h.Td(timeWithTitle(m.Created)),
			h.Td(actionNode),
		)
	})

	return h.Table(
		h.Class("table"),
		h.THead(h.Tr(
			h.Th(g.Text("Version")),
			h.Th(g.Text("Commit")),
			h.Th(g.Text("Description")),
			h.Th(g.Text("Last updated")),
			h.Th(g.Text("")),
		)),
		h.TBody(g.Group(rows)),
	)
}

func parseGHRef(raw []byte) *model.GHRef {
	if len(raw) == 0 {
		return nil
	}
	var ref model.GHRef
	if err := json.Unmarshal(raw, &ref); err != nil {
		return nil
	}
	if ref.Owner == "" && ref.Repo == "" && ref.Ref == "" {
		return nil
	}
	return &ref
}

func ghRefLink(ref *model.GHRef) g.Node {
	if ref == nil {
		return nil
	}

	shortSHA := ref.Ref
	if len(shortSHA) > 7 {
		shortSHA = shortSHA[:7]
	}

	label := shortSHA
	if ref.Repo != "" {
		label = ref.Repo + "@" + shortSHA
	}

	if ref.Owner != "" && ref.Repo != "" && ref.Ref != "" {
		href := fmt.Sprintf("https://github.com/%s/%s/commit/%s", ref.Owner, ref.Repo, ref.Ref)
		return h.A(h.Href(href), h.Target("_blank"), h.Class("gh-ref-link"), g.Text(label))
	}

	return g.Text(label)
}
