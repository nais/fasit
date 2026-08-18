package features

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	envpkg "github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/model"
	"github.com/nais/fasit/internal/reconciler"
	"github.com/nais/fasit/internal/ui/breadcrumb"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/nais/fasit/internal/ui/uidata"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type reconcileStatusRow struct {
	TenantName, EnvironmentName, EnvironmentID, State, Message string
	LastModified                                               time.Time
	DecidedAt                                                  time.Time
}

type matchingAssignment struct {
	ID, Version string
	Created     time.Time
}

func AssignmentDetailHandler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := loadFeatureData(r)
		if err != nil {
			handleFeatureLoadError(w, r, err)
			return
		}

		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			http.Error(w, "Failed to load assignment", http.StatusInternalServerError)
			return
		}

		d, err := featureassignment.Get(r.Context(), id)
		if err != nil {
			http.Error(w, "Failed to load assignment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		statuses, _ := reconciler.ReconcileStatuses(r.Context(), id)
		rows := make([]reconcileStatusRow, 0, len(statuses))
		for _, status := range statuses {
			env, err := envpkg.Get(r.Context(), status.EnvironmentID)
			if err != nil {
				continue
			}
			tenant, err := envpkg.GetTenant(r.Context(), env.TenantID)
			if err != nil {
				continue
			}
			rows = append(rows, reconcileStatusRow{
				TenantName:      tenant.Name,
				EnvironmentName: env.Name,
				EnvironmentID:   status.EnvironmentID.String(),
				State:           reconciler.NormalizeStatus(string(status.State)),
				Message:         status.Message,
				LastModified:    status.LastModified,
				DecidedAt:       status.DecidedAt,
			})
		}

		seenEnv := make(map[string]bool, len(rows))
		for _, row := range rows {
			seenEnv[row.EnvironmentID] = true
		}
		for _, es := range featureAssignmentEnvStatuses(r.Context(), data.CurrentFeature) {
			if es.FeatureAssignmentID != id.String() || !es.IsOverridden || seenEnv[es.EnvironmentID] {
				continue
			}
			rows = append(rows, reconcileStatusRow{
				TenantName:      es.TenantName,
				EnvironmentName: es.Name,
				EnvironmentID:   es.EnvironmentID,
				State:           "OVERRIDDEN",
			})
			seenEnv[es.EnvironmentID] = true
		}

		allInstructions, err := uidata.ListDeployInstructions(r.Context(), id)
		if err != nil {
			http.Error(w, "Failed to load deploy instructions: "+err.Error(), http.StatusInternalServerError)
			return
		}
		seen := make(map[string]bool)
		var instructions []*uidata.DeployInstruction
		for _, di := range allInstructions {
			envID := di.EnvironmentID.String()
			if !seen[envID] {
				seen[envID] = true
				instructions = append(instructions, di)
			}
		}

		allByFeature, _ := featureassignment.ListAllByFeature(r.Context(), d.Feature.Name)
		var matching []matchingAssignment
		var supersededBy *matchingAssignment
		for _, other := range allByFeature {
			if other.ID == d.ID {
				continue
			}
			if !maps.Equal(map[string]string(other.TargetLabels), map[string]string(d.TargetLabels)) {
				continue
			}
			m := matchingAssignment{
				ID:      other.ID.String(),
				Version: other.Feature.Version,
				Created: other.Created,
			}
			matching = append(matching, m)
			if other.Active && other.Created.After(d.Created) && (supersededBy == nil || other.Created.After(supersededBy.Created)) {
				m := m
				supersededBy = &m
			}
		}

		data.ActiveTab = "assignments"
		data.IsAssignmentDetail = true
		data.Assignment = d
		data.AssignmentStatusRows = rows
		data.AssignmentInstructions = instructions
		data.AssignmentMatching = matching
		data.AssignmentSupersededBy = supersededBy

		featureName := d.Feature.Name
		data.Breadcrumbs = []breadcrumb.Crumb{
			breadcrumb.Features(),
			breadcrumb.Feature(featureName),
			{Label: "Assignments", URL: "/features/" + featureName + "/assignments"},
			{Label: d.Feature.Version},
		}

		renderPage(w, r, layout.Props{
			Title:       fmt.Sprintf("%s %s", featureName, d.Feature.Version),
			CurrentPage: components.PageFeatures,
			Content:     detailPage(data),
		})
	}
}

func assignmentDetailPageContent(data *DetailPage) g.Node {
	d := data.Assignment
	featureName := d.Feature.Name

	meta := []g.Node{
		metaRow("Chart", g.Text(d.Feature.Chart)),
		metaRow("Target", assignmentTargetPills(assignmentTargetLabels(d))),
		metaRow("Version", g.Text(d.Feature.Version)),
		metaRow("Created", assignmentTimeWithTitle(d.Created)),
	}
	if d.Description != nil && *d.Description != "" {
		meta = append(meta, metaRow("Description", g.Text(*d.Description)))
	}
	if ref := parseGHRef(d.GHRef); ref != nil {
		meta = append(meta, metaRow("Commit", ghRefLink(ref)))
	}

	content := []g.Node{
		h.Div(
			h.Class("deploy-detail-topbar"),
			g.If(d.Active, h.Details(
				h.Class("env-actions"),
				h.Summary(h.Class("env-actions-toggle"), g.Attr("title", "Actions"), g.Text("\u22ee")),
				h.Div(
					h.Class("env-actions-menu"),
					h.Button(h.Type("button"), g.Attr("popovertarget", "set-version"), g.Text("Set version")),
					h.Button(h.Type("button"), h.Class("env-actions-danger"), g.Attr("popovertarget", "deactivate-assignment"), g.Text("Deactivate assignment")),
				),
			)),
		),
		g.If(d.Active, setVersionPopover("set-version", featureName, d.Feature.Chart, assignmentTargetLabels(d))),
		g.If(d.Active, deactivateAssignmentPopover(d)),
	}

	if !d.Active {
		content = append([]g.Node{
			h.Span(h.Class("inactive-badge"), g.Text("Inactive")),
		}, content...)
	}

	if data.AssignmentSupersededBy != nil {
		content = append(content, h.Div(
			h.Class("banner banner-warning"),
			h.P(
				g.Text("This is a previous version. Currently active: "),
				h.A(h.Href("/features/"+featureName+"/assignments/"+data.AssignmentSupersededBy.ID), g.Text(data.AssignmentSupersededBy.Version)),
			),
		))
	}

	content = append(content, h.Table(h.Class("table meta-table table-compact"), h.TBody(g.Group(meta))))

	type envRow struct {
		TenantName, EnvironmentName, EnvironmentID, State, Message string
		LastModified                                               time.Time
		DecidedAt                                                  time.Time
	}
	envRows := make(map[string]*envRow)
	for _, s := range data.AssignmentStatusRows {
		envRows[s.EnvironmentID] = &envRow{
			TenantName:      s.TenantName,
			EnvironmentName: s.EnvironmentName,
			EnvironmentID:   s.EnvironmentID,
			State:           s.State,
			Message:         s.Message,
			LastModified:    s.LastModified,
			DecidedAt:       s.DecidedAt,
		}
	}
	for _, di := range data.AssignmentInstructions {
		envID := di.EnvironmentID.String()
		if _, ok := envRows[envID]; !ok {
			envRows[envID] = &envRow{
				TenantName:      di.TenantName,
				EnvironmentName: di.EnvironmentName,
				EnvironmentID:   envID,
				LastModified:    di.LastModified,
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
		content = append(
			content,
			h.H2(g.Text("Instances")),
			h.Table(
				h.Class("table sortable"),
				g.Attr("data-sort-key", "assignment-detail-envs"),
				h.THead(h.Tr(
					h.Th(g.Text("Tenant")),
					h.Th(g.Text("Environment")),
					h.Th(g.Text("Status")),
					h.Th(g.Text("When")),
					h.Th(g.Text("Reconcile decision")),
					h.Th(g.Text("Since")),
					h.Th(g.Text("")),
				)),
				h.TBody(g.Group(g.Map(sortedEnvRows, func(r *envRow) g.Node {
					logsCell := g.Node(g.Text(""))
					if r.State != "OVERRIDDEN" {
						logsCell = h.A(h.Href("/assignments/"+d.ID.String()+"/logs/"+r.EnvironmentID), g.Text("View logs"))
					}
					return h.Tr(
						h.Td(g.Text(r.TenantName)),
						h.Td(h.A(h.Href("/features/"+featureName+"/envs/"+r.TenantName+"/"+r.EnvironmentName), g.Text(r.EnvironmentName))),
						h.Td(components.Status(r.State)),
						view.TimeCell(r.LastModified),
						h.Td(g.Text(r.Message)),
						view.TimeCell(r.DecidedAt),
						h.Td(logsCell),
					)
				}))),
			),
		)
	}

	if len(data.AssignmentMatching) > 0 {
		content = append(
			content,
			h.H2(g.Text("Previous versions")),
			h.Table(
				h.Class("table"),
				h.THead(h.Tr(
					h.Th(g.Text("Version")),
					h.Th(g.Text("Created")),
				)),
				h.TBody(g.Group(g.Map(data.AssignmentMatching, func(m matchingAssignment) g.Node {
					return h.Tr(
						h.Td(h.A(h.Href("/features/"+featureName+"/assignments/"+m.ID), g.Text(m.Version))),
						view.TimeCell(m.Created),
					)
				}))),
			),
		)
	}

	return h.Div(g.Group(content))
}

func assignmentTimeWithTitle(t time.Time) g.Node {
	if t.IsZero() {
		return g.Text("")
	}
	return h.Span(h.Title(view.FormatTime(t)), g.Text(view.RelativeTime(t)))
}

func assignmentTargetLabels(d *featureassignment.FeatureAssignment) map[string]string {
	labels := d.Target()
	if len(labels) == 0 {
		return nil
	}
	out := make(map[string]string, len(labels))
	for _, label := range labels {
		out[label.Key] = label.Value
	}
	return out
}

func assignmentTargetPills(labels map[string]string) g.Node {
	if len(labels) == 0 {
		return h.Span(h.Class("label-filter-tag"), g.Text("All environments"))
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pills := make([]g.Node, 0, len(keys))
	for _, k := range keys {
		pills = append(pills, h.Span(h.Class("label-filter-tag"), g.Text(k+": "+labels[k])))
	}
	return g.Group(pills)
}

func deactivateAssignmentPopover(d *featureassignment.FeatureAssignment) g.Node {
	return components.Popover(
		"deactivate-assignment", "", "Deactivate assignment",
		h.P(g.Textf("This will deactivate %s. It will no longer be reconciled.", d.Feature.Name)),
		h.Form(
			h.Method("POST"), h.Action("/assignments/"+d.ID.String()+"/deactivate"),
			h.Input(h.Type("hidden"), h.Name("redirect"), h.Value("/features/"+d.Feature.Name+"/assignments")),
			components.PopoverActions(
				h.Button(h.Type("submit"), g.Text("Deactivate")),
			),
		),
	)
}

func parseGHRef(raw []byte) *model.GitHubCommit {
	if len(raw) == 0 {
		return nil
	}
	var ref model.GitHubCommit
	if err := json.Unmarshal(raw, &ref); err != nil {
		return nil
	}
	if ref.Owner == "" && ref.Repo == "" && ref.Ref == "" {
		return nil
	}
	return &ref
}

func ghRefLink(ref *model.GitHubCommit) g.Node {
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
