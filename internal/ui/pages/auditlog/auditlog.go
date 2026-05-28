package auditlog

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/ui/breadcrumb"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, *http.Request, layout.Props)

func Handler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		var entries []*audit.Entry
		var err error
		if query == "" {
			entries, err = audit.ListRecent(r.Context(), 200)
		} else {
			entries, err = audit.SearchRecent(r.Context(), strings.Fields(query), 200)
		}
		if err != nil {
			http.Error(w, "Failed to load audit log", http.StatusInternalServerError)
			return
		}

		renderPage(w, r, layout.Props{
			Title:       "Activity Log",
			CurrentPage: components.PageDeployments,
			Content:     listPage(entries, query),
		})
	}
}

func listPage(entries []*audit.Entry, query string) g.Node {
	return h.Div(
		h.Class("container"),
		h.Main(
			h.Class("main-content"),
			components.Breadcrumbs([]breadcrumb.Crumb{{Label: "Activity Log", URL: "/auditlog"}}),
			h.Div(h.Class("card"), h.Div(h.Class("card-body"),
				h.Div(h.Class("deployments-header"),
					h.Div(h.Class("deployments-toolbar"),
						h.Input(
							h.Type("search"),
							h.Class("table-filter"),
							h.Name("q"),
							h.Placeholder("Filter by action, resource, environment, actor…"),
							g.Attr("aria-label", "Filter activity log"),
							g.Attr("autocomplete", "off"),
							g.Attr("data-url-filter", "q"),
							g.If(query != "", h.Value(query)),
						),
					),
				),
				activityTable(entries),
			)),
		),
	)
}

func activityTable(entries []*audit.Entry) g.Node {
	if len(entries) == 0 {
		return h.P(h.Class("text-muted"), g.Text("No activity recorded."))
	}

	rows := make([]g.Node, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, h.Tr(
			h.Td(g.Text(string(e.Action))),
			h.Td(ResourceLink(e)),
			h.Td(EnvLink(e)),
			h.Td(h.Class("text-muted"), g.Text(Description(e))),
			h.Td(view.ActorNode(e.Actor)),
			h.Td(h.Class("text-muted"), h.Title(view.FormatTime(e.CreatedAt)), g.Text(view.RelativeTime(e.CreatedAt))),
		))
	}

	return h.Table(h.Class("table sortable"), g.Attr("data-sort-key", "auditlog"),
		h.THead(h.Tr(
			h.Th(g.Text("Action")),
			h.Th(g.Text("Resource")),
			h.Th(g.Text("Environment")),
			h.Th(g.Text("Details")),
			h.Th(g.Text("Actor")),
			h.Th(g.Text("When")),
		)),
		h.TBody(g.Group(rows)),
	)
}

func Description(e *audit.Entry) string {
	if e.ObjectType == audit.ObjectTypeDeployment && e.Action == audit.ActionCreated {
		description := strings.TrimSpace(e.Description)
		version := strings.TrimSpace(strings.TrimPrefix(strings.Split(description, "→")[0], "version"))
		if version == "" {
			return description
		}
		return "version " + version + " → " + auditTargetDescription(e)
	}
	if e.Description != "" {
		return e.Description
	}
	return e.Summary()
}

func auditTargetDescription(e *audit.Entry) string {
	var metadata struct {
		Target map[string]string `json:"target"`
	}
	if len(e.Metadata) > 0 && json.Unmarshal(e.Metadata, &metadata) == nil && len(metadata.Target) > 0 {
		return formatLabels(metadata.Target)
	}
	parts := strings.Split(e.Description, "→")
	if len(parts) > 1 {
		if target := strings.TrimSpace(parts[1]); target != "" {
			return target
		}
	}
	return "all environments"
}

func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return "all environments"
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+labels[k])
	}
	return strings.Join(parts, ", ")
}

func ResourceLink(e *audit.Entry) g.Node {
	var nodes []g.Node

	// Deployment with metadata deploymentId: "deployment of feature" with two links
	if e.ObjectType == audit.ObjectTypeDeployment && e.ObjectID != "all" {
		if depID := metadataString(e.Metadata, "deploymentId"); depID != "" {
			nodes = append(nodes,
				h.A(h.Href("/deployments/"+depID), g.Text("deployment")),
				g.Text(" of "),
				h.A(h.Href("/features/"+e.ObjectID), g.Text(e.ObjectID)),
			)
		} else if e.Action == audit.ActionTriggered {
			nodes = append(nodes,
				g.Text("re-deployment of "),
				h.A(h.Href("/features/"+e.ObjectID), g.Text(e.ObjectID)),
			)
		}
	}

	if len(nodes) == 0 {
		label := e.ObjectType.Display() + " " + e.ObjectID
		href := ResourceHref(e)
		if href == "" {
			nodes = append(nodes, g.Text(label))
		} else {
			nodes = append(nodes, h.A(h.Href(href), g.Text(label)))
		}
	}

	return g.Group(nodes)
}

func ResourceHref(e *audit.Entry) string {
	switch e.ObjectType {
	case audit.ObjectTypeFeature:
		return "/features/" + e.ObjectID
	case audit.ObjectTypeDeployment:
		if e.ObjectID == "all" {
			return ""
		}
		if _, err := uuid.Parse(e.ObjectID); err == nil {
			return ""
		}
		return "/features/" + e.ObjectID
	case audit.ObjectTypeConfiguration:
		// ObjectID is "feature/key" — link to the feature
		if i := strings.IndexByte(e.ObjectID, '/'); i > 0 {
			return "/features/" + e.ObjectID[:i]
		}
		return ""
	case audit.ObjectTypeEnvironment, audit.ObjectTypeEnvironmentValue:
		if e.TenantName != "" && e.EnvironmentName != "" {
			return "/tenants/" + e.TenantName + "/envs/" + e.EnvironmentName
		}
		return ""
	default:
		return ""
	}
}

func EnvLink(e *audit.Entry) g.Node {
	if e.TenantName == "" || e.EnvironmentName == "" {
		return g.Text("")
	}
	label := e.TenantName + "/" + e.EnvironmentName
	href := "/tenants/" + e.TenantName + "/envs/" + e.EnvironmentName
	return h.A(h.Href(href), g.Text(label))
}

func metadataString(meta []byte, key string) string {
	if len(meta) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(meta, &m); err != nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}
