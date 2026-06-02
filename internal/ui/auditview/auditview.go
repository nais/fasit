package auditview

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/ui/components"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// DisplayAction returns the user-facing label for an audit entry's action.
func DisplayAction(e *audit.Entry) string {
	if isRedeploy(e) {
		return "redeploy"
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
			nodes = append(nodes,
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
				return "/tenants/" + e.TenantName + "/envs/" + e.EnvironmentName + "/features/" + feature + "/config#config-" + key
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

// DetailNode renders the details cell content for an audit entry.
func DetailNode(e *audit.Entry) g.Node {
	desc := Description(e)
	showDesc := desc != "" && !IsDescriptionRedundant(e)
	return g.Group([]g.Node{
		components.ConfigChangeNode(e),
		g.If(showDesc, h.Div(g.Text(desc))),
	})
}

func isRedeploy(e *audit.Entry) bool {
	return e.ObjectType == audit.ObjectTypeFeatureAssignment &&
		(e.Action == audit.ActionTriggered || e.Action == audit.ActionRedeploy)
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
