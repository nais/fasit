package rollouts

import (
	"fmt"
	"maps"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/database/types"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/deployment/deploymentsql"
	envpkg "github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/ui/breadcrumb"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type deploymentStatusRow struct {
	TenantName, EnvironmentName, EnvironmentID, State, Message, LastModified string
}

func DeleteDeploymentHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			http.Error(w, "Failed to parse deployment ID", http.StatusInternalServerError)
			return
		}

		if err := deployment.DeleteDeployment(r.Context(), id); err != nil {
			http.Error(w, "Failed to delete deployment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/ui/deployments", http.StatusSeeOther)
	}
}

type matchingDeployment struct {
	ID, Version, Created string
}

func DeleteDeploymentsByFeatureAndTargetHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			http.Error(w, "Failed to parse deployment ID", http.StatusInternalServerError)
			return
		}

		dep, err := deployment.GetDeployment(r.Context(), id)
		if err != nil {
			http.Error(w, "Failed to load deployment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := deployment.DeleteDeploymentsByFeatureAndTarget(r.Context(), dep.Feature.Name, types.EnvironmentLabels(dep.TargetLabels)); err != nil {
			http.Error(w, "Failed to delete deployments: "+err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/ui/deployments", http.StatusSeeOther)
	}
}

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
				EnvironmentID:   status.EnvironmentID.String(),
				State:           status.State.String(),
				Message:         status.Message,
				LastModified:    formatTime(status.LastModified),
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
				Created: formatTime(d.Created),
			})
		}

		renderPage(w, layout.Props{Title: fmt.Sprintf("Deployment %s v%s", dep.Feature.Name, dep.Feature.Version), CurrentPage: components.PageDeployments, Content: deploymentPage(dep, rows, deployInstructions, matching)})
	}
}

func deploymentPage(d *deployment.Deployment, statuses []deploymentStatusRow, deployInstructions []deploymentsql.ListDeployInstructionsByDeploymentIDRow, matching []matchingDeployment) g.Node {
	meta := []g.Node{
		metaRow("Feature", h.A(h.Href("/ui/features/"+d.Feature.Name), g.Text(d.Feature.Name))),
		metaRow("Version", g.Text(d.Feature.Version)),
		metaRow("Target", g.Text(deploymentTarget(d))),
		metaRow("Created", g.Text(formatTime(d.Created))),
	}

	if d.Description != nil && *d.Description != "" {
		meta = append(meta, metaRow("Description", g.Text(*d.Description)))
	}

	meta = append(meta, metaRow("Actions", h.Form(
		h.Method("POST"),
		h.Action("/ui/deployments/"+d.ID.String()+"/delete"),
		h.Button(h.Type("submit"), h.Class("btn-small"), g.Attr("onclick", "return confirm('Are you sure?')"), g.Text("Delete")),
	)))

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
				h.Th(g.Text("Logs")),
			)),
			h.TBody(g.Group(g.Map(statuses, func(s deploymentStatusRow) g.Node {
				return h.Tr(
					h.Td(h.A(h.Href("/ui/tenants/"+s.TenantName), g.Text(s.TenantName))),
					h.Td(h.A(h.Href("/ui/tenants/"+s.TenantName+"/envs/"+s.EnvironmentName+"/"+d.Feature.Name), g.Text(s.EnvironmentName))),
					h.Td(rolloutStatus(s.State)),
					h.Td(g.Text(s.Message)),
					h.Td(g.Text(s.LastModified)),
					h.Td(h.A(h.Href("/ui/deployments/"+d.ID.String()+"/logs/"+s.EnvironmentID), g.Attr("title", "View logs"), g.Text("📋"))),
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
					h.Td(h.A(h.Href("/ui/deployments/"+m.ID), g.Text(m.Version))),
					h.Td(g.Text(m.Created)),
				)
			}))),
		),
		h.Form(
			h.Method("POST"),
			h.Action("/ui/deployments/"+d.ID.String()+"/delete-matching"),
			h.Button(h.Type("submit"), h.Class("btn-small"), g.Attr("onclick", "return confirm('Are you sure?')"), g.Text("Delete all deployments")),
			g.Textf(" (%d)", len(matching)),
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
					h.Td(h.A(h.Href("/ui/tenants/"+di.TenantName+"/envs/"+di.EnvironmentName), g.Textf("%s / %s", di.TenantName, di.EnvironmentName))),
					h.Td(rolloutStatus(strings.ToUpper(di.DeployInstruction.Status))),
					h.Td(g.Text(formatTime(di.DeployInstruction.Created.Time))),
					h.Td(g.Text(formatTime(di.DeployInstruction.LastModified.Time))),
					h.Td(h.A(h.Href("/ui/deployments/"+d.ID.String()+"/logs/"+di.DeployInstruction.EnvironmentID.String()), g.Attr("title", "View logs"), g.Text("📋"))),
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

func DeploymentLogsHandler(renderPage RenderPage) http.HandlerFunc {
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
		renderPage(w, layout.Props{
			Title:       title,
			CurrentPage: components.PageDeployments,
			Content:     deploymentLogsPage(dep, log),
		})
	}
}

type featureGroup struct {
	FeatureName string
	LatestTime  time.Time
	Targets     []Summary
}

func groupDeployments(items []Summary) []featureGroup {
	type key struct{ feature, target string }
	latest := make(map[key]Summary)
	for _, item := range items {
		k := key{item.FeatureName, item.Target}
		if existing, ok := latest[k]; !ok || item.createdAt.After(existing.createdAt) {
			latest[k] = item
		}
	}

	groupMap := make(map[string]*featureGroup)
	for _, item := range latest {
		fg, ok := groupMap[item.FeatureName]
		if !ok {
			groupMap[item.FeatureName] = &featureGroup{FeatureName: item.FeatureName, LatestTime: item.createdAt, Targets: []Summary{item}}
			continue
		}
		fg.Targets = append(fg.Targets, item)
		if item.createdAt.After(fg.LatestTime) {
			fg.LatestTime = item.createdAt
		}
	}

	groups := make([]featureGroup, 0, len(groupMap))
	for _, fg := range groupMap {
		sort.Slice(fg.Targets, func(i, j int) bool {
			return fg.Targets[i].createdAt.After(fg.Targets[j].createdAt)
		})
		groups = append(groups, *fg)
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].LatestTime.After(groups[j].LatestTime)
	})

	return groups
}

func DeploymentsHandler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var items []Summary

		deployments, err := deployment.ListDeployments(r.Context())
		if err == nil {
			for _, dep := range deployments {
				statuses, _ := deployment.ListDeploymentStatuses(r.Context(), dep.ID)
				status, disabledCount := aggregateDeploymentStatus(statuses)
				items = append(items, Summary{
					FeatureName:   dep.Feature.Name,
					Version:       dep.Feature.Version,
					Status:        status,
					Target:        deploymentTarget(dep),
					Created:       formatTime(dep.Created),
					Completed:     latestStatusTime(statuses),
					DeploymentID:  dep.ID.String(),
					createdAt:     dep.Created,
					disabledCount: disabledCount,
				})
			}
		}

		groups := groupDeployments(items)

		renderPage(w, layout.Props{
			Title:       "Deployments",
			CurrentPage: components.PageDeployments,
			Content:     deploymentsPage(groups),
		})
	}
}

func deploymentsPage(groups []featureGroup) g.Node {
	return h.Div(
		h.Class("container"),
		h.Main(
			h.Class("main-content"),
			components.Breadcrumbs([]breadcrumb.Crumb{breadcrumb.Deployments()}),
			h.Div(
				h.Class("card"),
				h.Div(
					h.Class("card-body"),
					h.H1(g.Text("Deployments")),
					deploymentsContent(groups),
				),
			),
		),
	)
}

func deploymentsContent(groups []featureGroup) g.Node {
	if len(groups) == 0 {
		return h.P(g.Text("No deployments yet."))
	}

	nodes := make([]g.Node, len(groups))
	for i, fg := range groups {
		isFirst := i == 0
		nodes[i] = deploymentGroup(fg, isFirst)
	}
	return h.Div(h.Class("deployment-groups"), g.Group(nodes))
}

func deploymentGroup(fg featureGroup, open bool) g.Node {
	detailsAttrs := []g.Node{
		h.Class("deployment-group"),
	}
	if open {
		detailsAttrs = append(detailsAttrs, g.Attr("open"))
	}

	rows := make([]g.Node, len(fg.Targets))
	for i, dep := range fg.Targets {
		rows[i] = h.Div(
			h.Class("deployment-row"),
			h.Span(h.Class("dep-target"), g.Text(dep.Target)),
			h.Span(h.Class("dep-version"), versionCell(dep)),
			h.Span(h.Class("dep-status"), statusCell(dep)),
			h.Span(h.Class("dep-time"), g.Text(dep.Created)),
		)
	}

	return h.Details(
		g.Group(detailsAttrs),
		h.Summary(
			h.Class("deployment-group-summary"),
			h.A(h.Href("/ui/features/"+fg.FeatureName), g.Text(fg.FeatureName)),
			h.Span(h.Class("dep-group-time"), g.Text(formatTime(fg.LatestTime))),
		),
		h.Div(h.Class("deployment-group-body"), g.Group(rows)),
	)
}

func aggregateDeploymentStatus(statuses []*deployment.DeploymentStatus) (string, int) {
	if len(statuses) == 0 {
		return "UNKNOWN", 0
	}

	disabledCount := 0
	for _, s := range statuses {
		if s.State == deployment.DeploymentStatusStateDisabled {
			disabledCount++
		}
	}

	if disabledCount == len(statuses) {
		return "DISABLED", disabledCount
	}

	allDeployed := true
	for _, s := range statuses {
		if s.State == deployment.DeploymentStatusStateDisabled {
			continue
		}
		switch s.State {
		case deployment.DeploymentStatusStateFailed:
			return "FAILED", disabledCount
		case deployment.DeploymentStatusStatePending, deployment.DeploymentStatusStateCreated:
			allDeployed = false
		case deployment.DeploymentStatusStateDeployed:
		default:
			allDeployed = false
		}
	}

	if allDeployed {
		return "DEPLOYED", disabledCount
	}

	return "PENDING", disabledCount
}

func latestStatusTime(statuses []*deployment.DeploymentStatus) string {
	var latest time.Time
	for _, s := range statuses {
		if s.LastModified.After(latest) {
			latest = s.LastModified
		}
	}

	if latest.IsZero() {
		return ""
	}

	return formatTime(latest)
}

func deploymentTarget(dep *deployment.Deployment) string {
	labels := dep.Target()
	if len(labels) == 0 {
		return "All environments"
	}

	parts := make([]string, 0, len(labels))
	for _, label := range labels {
		parts = append(parts, label.Key+"="+label.Value)
	}

	return strings.Join(parts, ", ")
}

func deploymentLogsPage(dep *deployment.Deployment, log *model.RolloutLog) g.Node {
	var logContent []g.Node

	if log == nil || len(log.Lines) == 0 {
		logContent = append(logContent, h.P(g.Text("No logs available.")))
	} else {
		var sb strings.Builder
		for i, line := range log.Lines {
			if i > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(formatTime(line.Timestamp))
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
