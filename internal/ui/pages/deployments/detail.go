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
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/deployment/deploymentsql"
	envpkg "github.com/nais/fasit/internal/environment"
	commonmodel "github.com/nais/fasit/internal/model"
	"github.com/nais/fasit/internal/ui/breadcrumb"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
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

func DetailHandler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			http.Error(w, "Failed to load deployment", http.StatusInternalServerError)
			return
		}

		dep, err := deployment.Get(r.Context(), id)
		if err != nil {
			http.Error(w, "Failed to load deployment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		statuses, _ := deployment.ListDeploymentStatuses(r.Context(), id)

		rows := make([]deploymentStatusRow, 0, len(statuses))
		for _, status := range statuses {
			env, err := envpkg.Get(r.Context(), status.EnvironmentID)
			if err != nil {
				continue
			}

			tenant, err := envpkg.GetTenant(r.Context(), env.TenantID)
			if err != nil {
				continue
			}

			state := deployment.NormalizeStatus(status.State.String())
			rows = append(rows, deploymentStatusRow{
				TenantName:      tenant.Name,
				EnvironmentName: env.Name,
				EnvironmentID:   status.EnvironmentID.String(),
				State:           state,
				Message:         status.Message,
				LastModified:    status.LastModified,
			})
		}

		allDeployInstructions, err := deployment.ListDeployInstructions(r.Context(), id)
		if err != nil {
			http.Error(w, "Failed to load deploy instructions: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Deduplicate to latest instruction per environment (query is ordered by created DESC).
		seen := make(map[string]bool)
		var deployInstructions []deploymentsql.ListDeployInstructionsRow
		for _, di := range allDeployInstructions {
			envID := di.DeployInstruction.EnvironmentID.String()
			if !seen[envID] {
				seen[envID] = true
				deployInstructions = append(deployInstructions, di)
			}
		}

		allByFeature, _ := deployment.ListAllByFeature(r.Context(), dep.Feature.Name)
		var matching []matchingDeployment
		var supersededBy *matchingDeployment
		for _, d := range allByFeature {
			if d.ID == dep.ID {
				continue
			}
			if !maps.Equal(map[string]string(d.TargetLabels), map[string]string(dep.TargetLabels)) {
				continue
			}
			m := matchingDeployment{
				ID:      d.ID.String(),
				Version: d.Feature.Version,
				Created: d.Created,
			}
			matching = append(matching, m)
			if d.Active && d.Created.After(dep.Created) && (supersededBy == nil || d.Created.After(supersededBy.Created)) {
				copy := m
				supersededBy = &copy
			}
		}

		renderPage(w, r, layout.Props{
			Title:       fmt.Sprintf("Deployment %s %s", dep.Feature.Name, dep.Feature.Version),
			CurrentPage: components.PageDeployments,
			Content:     detailPage(dep, rows, deployInstructions, matching, supersededBy),
		})
	}
}

func detailPage(d *deployment.Deployment, statuses []deploymentStatusRow, deployInstructions []deploymentsql.ListDeployInstructionsRow, matching []matchingDeployment, supersededBy *matchingDeployment) g.Node {
	meta := []g.Node{
		metaRow("Chart", g.Text(d.Feature.Chart)),
		metaRow("Target", targetPills(deploymentTargetLabels(d))),
		metaRow("Version", g.Text(d.Feature.Version)),
		metaRow("Created", timeWithTitle(d.Created)),
	}

	if d.Description != nil && *d.Description != "" {
		meta = append(meta, metaRow("Description", g.Text(*d.Description)))
	}

	if ref := parseGHRef(d.GHRef); ref != nil {
		meta = append(meta, metaRow("Commit", ghRefLink(ref)))
	}

	content := []g.Node{
		h.Div(h.Class("deploy-detail-topbar"),
			g.If(d.Active, h.Details(h.Class("env-actions"),
				h.Summary(h.Class("env-actions-toggle"), g.Attr("title", "Actions"), g.Text("\u22ee")),
				h.Div(h.Class("env-actions-menu"),
					h.Button(h.Type("button"), g.Attr("popovertarget", "set-version"), g.Text("Set version")),
					h.Button(h.Type("button"), h.Class("env-actions-danger"), g.Attr("popovertarget", "deactivate-deployment"), g.Text("Deactivate deployment")),
				),
			)),
		),
		g.If(d.Active, setVersionPopover(d)),
		g.If(d.Active, deactivateDeploymentPopover(d)),
	}

	if !d.Active {
		content = append(content, h.Div(h.Class("banner banner-inactive"),
			h.P(g.Text("This deployment is inactive. It is no longer being reconciled.")),
		))
	}

	if supersededBy != nil {
		content = append(content, h.Div(h.Class("banner banner-warning"),
			h.P(
				g.Text("This is a previous version. Currently active: "),
				h.A(h.Href("/deployments/"+supersededBy.ID), g.Text(supersededBy.Version)),
			),
		))
	}

	content = append(content, h.Table(h.Class("table meta-table table-compact"), h.TBody(g.Group(meta))))

	// Environments — merged view of deployment statuses and instructions
	type envRow struct {
		TenantName      string
		EnvironmentName string
		EnvironmentID   string
		State           string
		Message         string
		LastModified    time.Time
	}

	envRows := make(map[string]*envRow)
	for _, s := range statuses {
		envRows[s.EnvironmentID] = &envRow{
			TenantName:      s.TenantName,
			EnvironmentName: s.EnvironmentName,
			EnvironmentID:   s.EnvironmentID,
			State:           s.State,
			Message:         s.Message,
			LastModified:    s.LastModified,
		}
	}
	for _, di := range deployInstructions {
		envID := di.DeployInstruction.EnvironmentID.String()
		if _, ok := envRows[envID]; !ok {
			envRows[envID] = &envRow{
				TenantName:      di.TenantName,
				EnvironmentName: di.EnvironmentName,
				EnvironmentID:   envID,
				LastModified:    di.DeployInstruction.LastModified.Time,
			}
		}
	}

	var sortedEnvRows []*envRow
	for _, r := range envRows {
		sortedEnvRows = append(sortedEnvRows, r)
	}
	sort.Slice(sortedEnvRows, func(i, j int) bool {
		if sortedEnvRows[i].TenantName != sortedEnvRows[j].TenantName {
			return sortedEnvRows[i].TenantName < sortedEnvRows[j].TenantName
		}
		return sortedEnvRows[i].EnvironmentName < sortedEnvRows[j].EnvironmentName
	})

	if len(sortedEnvRows) > 0 {
		content = append(content,
			h.H2(g.Text("Instances")),
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
				h.TBody(g.Group(g.Map(sortedEnvRows, func(r *envRow) g.Node {
					return h.Tr(
						h.Td(g.Text(r.TenantName)),
						h.Td(h.A(h.Href("/tenants/"+r.TenantName+"/envs/"+r.EnvironmentName+"/"+d.Feature.Name), g.Text(r.EnvironmentName))),
						h.Td(components.Status(r.State)),
						h.Td(g.Text(r.Message)),
						h.Td(timeWithTitle(r.LastModified)),
						h.Td(h.A(h.Href("/deployments/"+d.ID.String()+"/logs/"+r.EnvironmentID), g.Text("View logs"))),
					)
				}))),
			),
		)
	}

	// Previous versions
	if len(matching) > 0 {
		content = append(content,
			h.H2(g.Text("Previous versions")),
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
		)
	}

	return h.Div(
		h.Class("container"),
		components.Breadcrumbs([]breadcrumb.Crumb{
			breadcrumb.Deployments(),
			{Label: d.Feature.Name, URL: "/features/" + d.Feature.Name},
			{Content: targetPills(deploymentTargetLabels(d))},
		}),
		h.Main(
			h.Class("main-content"),
			components.Card(content...),
		),
	)
}

func parseGHRef(raw []byte) *commonmodel.GitHubCommit {
	if len(raw) == 0 {
		return nil
	}
	var ref commonmodel.GitHubCommit
	if err := json.Unmarshal(raw, &ref); err != nil {
		return nil
	}
	if ref.Owner == "" && ref.Repo == "" && ref.Ref == "" {
		return nil
	}
	return &ref
}

func ghRefLink(ref *commonmodel.GitHubCommit) g.Node {
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

func deactivateDeploymentPopover(d *deployment.Deployment) g.Node {
	return h.Div(g.Attr("popover", ""), h.ID("deactivate-deployment"),
		h.H3(g.Text("Deactivate deployment")),
		h.P(g.Textf("This will deactivate %s. It will no longer be reconciled.", d.Feature.Name)),
		h.Form(h.Method("POST"), h.Action("/deployments/"+d.ID.String()+"/deactivate"),
			h.Div(h.Class("popover-actions"),
				h.Button(h.Type("submit"), g.Text("Deactivate")),
				h.Button(h.Type("button"), g.Attr("popovertarget", "deactivate-deployment"), g.Attr("popovertargetaction", "hide"), g.Text("Cancel")),
			),
		),
	)
}

func setVersionPopover(d *deployment.Deployment) g.Node {
	return h.Div(g.Attr("popover", ""), h.ID("set-version"),
		h.H3(g.Text("Set version")),
		h.Form(h.Method("POST"), h.Action("/deployments"),
			h.Input(h.Type("hidden"), h.Name("chart"), h.Value(d.Feature.Chart)),
			targetHiddenInputs(deploymentTargetLabels(d)),
			h.Label(g.Text("Version")),
			h.Input(h.Type("text"), h.Name("version"), g.Attr("required", ""), g.Attr("placeholder", "e.g. 2026-05-21-001"), h.Value(d.Feature.Version)),
			h.Div(h.Class("popover-actions"),
				h.Button(h.Type("submit"), g.Text("Deploy")),
				h.Button(h.Type("button"), g.Attr("popovertarget", "set-version"), g.Attr("popovertargetaction", "hide"), g.Text("Cancel")),
			),
		),
	)
}

func targetHiddenInputs(labels map[string]string) g.Node {
	inputs := make([]g.Node, 0, len(labels))
	for k, v := range labels {
		inputs = append(inputs, h.Input(h.Type("hidden"), h.Name("target_label"), h.Value(k+"="+v)))
	}
	return g.Group(inputs)
}
