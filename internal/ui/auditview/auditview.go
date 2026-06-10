package auditview

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// DisplayAction returns the user-facing label for an audit entry's action.
func DisplayAction(e *audit.Entry) string {
	if isRedeploy(e) {
		return "redeployed"
	}
	return string(e.Action)
}

// IsDescriptionRedundant returns true when the description text adds no
// information beyond what ACTION + RESOURCE already convey.
func IsDescriptionRedundant(e *audit.Entry) bool {
	if e.ObjectType == audit.ObjectTypeConfiguration {
		return true
	}
	if isRedeploy(e) {
		return true
	}
	return false
}

// Description returns a formatted description for display.
func Description(e *audit.Entry) string {
	if e.ObjectType == audit.ObjectTypeFeatureAssignment && e.Action == audit.ActionCreated {
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

// ResourceLink renders the resource column as a clickable link (or plain text).
func ResourceLink(e *audit.Entry) g.Node {
	var nodes []g.Node

	if e.ObjectType == audit.ObjectTypeFeatureAssignment && e.ObjectID != "all" {
		if depID := metadataString(e.Metadata, "deploymentId"); depID != "" {
			nodes = append(nodes,
				h.A(h.Href("/assignments/"+depID), g.Text("assignment")),
				g.Text(" of "),
				h.A(h.Href("/features/"+e.ObjectID), g.Text(e.ObjectID)),
			)
		} else if isRedeploy(e) {
			parts := []g.Node{h.A(h.Href("/features/"+e.ObjectID), g.Text(e.ObjectID))}
			if e.TenantName != "" && e.EnvironmentName != "" {
				parts = append(parts, g.Text(" in "+e.TenantName+"/"+e.EnvironmentName))
			}
			nodes = append(nodes, parts...)
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

// ResourceHref returns the URL for an audit entry's resource, or "" if none.
func ResourceHref(e *audit.Entry) string {
	switch e.ObjectType {
	case audit.ObjectTypeFeature:
		return "/features/" + e.ObjectID
	case audit.ObjectTypeFeatureAssignment:
		if e.ObjectID == "all" {
			return ""
		}
		if _, err := uuid.Parse(e.ObjectID); err == nil {
			return ""
		}
		return "/features/" + e.ObjectID
	case audit.ObjectTypeConfiguration:
		if i := strings.IndexByte(e.ObjectID, '/'); i > 0 {
			feature := e.ObjectID[:i]
			key := e.ObjectID[i+1:]
			if e.EnvironmentID != nil && e.TenantName != "" && e.EnvironmentName != "" {
				return "/features/" + feature + "/envs/" + e.TenantName + "/" + e.EnvironmentName + "/config#config-" + key
			}
			return "/features/" + feature + "/config#config-" + key
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

// EnvLink renders the environment column as a link.
func EnvLink(e *audit.Entry) g.Node {
	if e.TenantName == "" || e.EnvironmentName == "" {
		return g.Text("")
	}
	label := e.TenantName + "/" + e.EnvironmentName
	href := "/tenants/" + e.TenantName + "/envs/" + e.EnvironmentName
	return h.A(h.Href(href), g.Text(label))
}

// DetailNode renders the details cell content for an audit entry (table view).
func DetailNode(e *audit.Entry) g.Node {
	desc := Description(e)
	showDesc := desc != "" && !IsDescriptionRedundant(e)
	return g.Group([]g.Node{
		configCell(e),
		g.If(showDesc, h.Div(g.Text(desc))),
	})
}

// configCell renders the config value diff followed by the tenant/environment
// (or "global") the config applies to. For non-config entries it falls back to
// the plain diff node (nil).
func configCell(e *audit.Entry) g.Node {
	diff := ConfigChangeNode(e)
	if e.ObjectType != audit.ObjectTypeConfiguration {
		return diff
	}
	loc := "global"
	if e.EnvironmentID != nil && e.TenantName != "" && e.EnvironmentName != "" {
		loc = "in " + e.TenantName + "/" + e.EnvironmentName
	}
	locNode := h.Span(h.Class("text-muted"), g.Text(loc))
	if diff == nil {
		return locNode
	}
	return g.Group([]g.Node{diff, h.Span(h.Class("text-muted"), g.Text(" · ")), locNode})
}

// ActivityListParams configures the compact activity sidebar.
type ActivityListParams struct {
	Title       string
	FilterBadge string
	AllHref     string
	Entries     []*audit.Entry
	// ResourceNode overrides how the resource is rendered per entry.
	// If nil, uses ResourceLink.
	ResourceNode func(*audit.Entry) g.Node
}

// ActivityList renders a compact activity sidebar section.
func ActivityList(p ActivityListParams) g.Node {
	if len(p.Entries) == 0 {
		return nil
	}

	resourceFn := p.ResourceNode
	if resourceFn == nil {
		resourceFn = ResourceLink
	}

	items := make([]g.Node, 0, len(p.Entries))
	for _, e := range p.Entries {
		desc := Description(e)
		showDesc := desc != "" && !IsDescriptionRedundant(e)
		items = append(items, h.Li(
			h.Div(h.Class("activity-meta"),
				h.Span(
					g.Text(DisplayAction(e)),
					g.If(e.Actor != "", g.Group([]g.Node{g.Text(" by "), h.Span(h.Class("activity-actor"), view.ActorNode(e.Actor))})),
				),
				h.Span(h.Title(view.FormatTime(e.CreatedAt)), g.Text(view.RelativeTime(e.CreatedAt))),
			),
			h.Div(h.Class("activity-resource"), resourceFn(e)),
			configCell(e),
			g.If(showDesc, h.Div(h.Class("activity-description"), g.Text(desc))),
		))
	}

	headerContent := []g.Node{
		h.Div(
			h.H3(g.Text(p.Title)),
			g.If(p.FilterBadge != "", h.Span(h.Class("activity-filter-badge"), g.Text(p.FilterBadge))),
		),
	}
	if p.AllHref != "" {
		headerContent = append(headerContent, h.A(h.Href(p.AllHref), h.Class("link-muted"), g.Text("All →")))
	}

	return h.Section(h.Class("activity-section"),
		h.Div(h.Class("activity-header"), g.Group(headerContent)),
		h.Ul(h.Class("activity-list"), g.Group(items)),
	)
}

// ConfigChangeNode renders a compact old→new diff for config audit entries.
func ConfigChangeNode(e *audit.Entry) g.Node {
	if e.ObjectType != audit.ObjectTypeConfiguration {
		return nil
	}
	if len(e.Metadata) == 0 {
		return nil
	}
	var meta map[string]string
	if json.Unmarshal(e.Metadata, &meta) != nil {
		return nil
	}
	if meta["secret"] == "true" {
		return h.Span(h.Class("text-muted"), g.Text("(secret)"))
	}
	oldVal, hasOld := meta["old"]
	newVal, hasNew := meta["new"]
	if !hasOld && !hasNew {
		return nil
	}
	oldClean := cleanVal(oldVal)
	newClean := cleanVal(newVal)
	if !hasOld {
		return h.Code(h.Title(newClean), g.Text(truncateVal(newClean)))
	}
	if !hasNew {
		return h.Code(h.Class("val-deleted"), h.Title(oldClean), g.Text(truncateVal(oldClean)))
	}
	return h.Span(h.Class("config-change"), h.Title(oldClean+" → "+newClean),
		h.Code(h.Class("val-old"), g.Text(truncateVal(oldClean))),
		g.Text(" → "),
		h.Code(g.Text(truncateVal(newClean))),
	)
}

func isRedeploy(e *audit.Entry) bool {
	return e.ObjectType == audit.ObjectTypeFeatureAssignment &&
		(e.Action == audit.ActionTriggered || e.Action == audit.ActionRedeploy)
}

func cleanVal(s string) string {
	s = strings.TrimPrefix(s, `"`)
	s = strings.TrimSuffix(s, `"`)
	return s
}

func truncateVal(s string) string {
	const max = 24
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
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
