package deployments

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nais/fasit/internal/ui/layout"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, *http.Request, layout.Props)

type Summary struct {
	FeatureName, Chart, Version, Status, Target, Created, Completed, DeploymentID string
	TargetLabels                                                                  map[string]string
	createdAt                                                                     time.Time
	disabledCount                                                                 int
}

func statusCell(s Summary) g.Node {
	if s.Status == "DEPLOYED" && s.disabledCount > 0 {
		return g.Group([]g.Node{
			h.Span(h.Class("status-success"), g.Attr("title", fmt.Sprintf("Disabled in %d environment(s)", s.disabledCount)), g.Attr("style", "font-size:inherit;line-height:inherit"), g.Text("⚠️")),
			g.Text(" Deployed"),
		})
	}
	return renderStatus(s.Status)
}

func renderStatus(status string) g.Node {
	switch strings.ToUpper(status) {
	case "DEPLOYED":
		return g.Group([]g.Node{h.Span(h.Class("status-success"), g.Text("✓")), g.Text(" Deployed")})
	case "FAILED":
		return g.Group([]g.Node{h.Span(h.Class("status-error"), g.Text("✗")), g.Text(" Failed")})
	case "PENDING", "PENDING-INSTALL", "PENDING-UPGRADE", "PENDING-ROLLBACK":
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
	default:
		return g.Text(status)
	}
}

func metaRow(label string, value g.Node) g.Node {
	return h.Tr(h.Td(h.Class("th-like"), g.Text(label)), h.Td(value))
}

func timeWithTitle(t time.Time) g.Node {
	if t.IsZero() {
		return g.Text("")
	}
	return h.Span(g.Attr("title", view.FormatTime(t)), g.Text(view.RelativeTime(t)))
}
