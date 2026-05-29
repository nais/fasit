package components

import (
	"strings"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func Status(status string) g.Node {
	switch strings.ToUpper(status) {
	case "DEPLOYED":
		return g.Group([]g.Node{h.Span(h.Class("status-success"), g.Text("✓")), g.Text(" Deployed")})
	case "FAILED":
		return g.Group([]g.Node{h.Span(h.Class("status-error"), g.Text("✗")), g.Text(" Failed")})
	case "PENDING", "PENDING-INSTALL", "PENDING-UPGRADE", "PENDING-ROLLBACK", "INVALIDATED":
		return g.Group([]g.Node{h.Span(h.Class("status-pending"), g.Text("⏳")), g.Text(" Pending")})
	case "DISABLED":
		return g.Group([]g.Node{h.Span(h.Class("status-disabled"), g.Text("○")), g.Text(" Disabled")})
	case "CREATED":
		return g.Group([]g.Node{h.Span(h.Class("status-pending"), g.Text("⏳")), g.Text(" Created")})
	case "UNKNOWN":
		return g.Group([]g.Node{h.Span(h.Class("status-pending"), g.Text("?")), g.Text(" Unknown")})
	case "ENABLED":
		return g.Group([]g.Node{h.Span(h.Class("status-success"), g.Text("✓")), g.Text(" Enabled")})
	case "OVERRIDDEN":
		return g.Group([]g.Node{h.Span(h.Class("status-disabled"), g.Text("⊘")), g.Text(" Overridden")})
	case "INACTIVE":
		return g.Group([]g.Node{h.Span(h.Class("status-disabled"), g.Text("◇")), g.Text(" Inactive")})
	default:
		return g.Text(status)
	}
}

func StatusClass(status string) string {
	switch strings.ToUpper(status) {
	case "DEPLOYED", "ENABLED":
		return "status-success"
	case "FAILED":
		return "status-error"
	case "PENDING", "PENDING-INSTALL", "PENDING-UPGRADE", "PENDING-ROLLBACK", "INVALIDATED", "CREATED":
		return "status-pending"
	case "DISABLED", "OVERRIDDEN", "INACTIVE":
		return "status-disabled"
	default:
		return "text-muted"
	}
}
