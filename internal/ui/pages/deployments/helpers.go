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
	FeatureName, Version, Status, Target, Created, Completed, DeploymentID string
	TargetLabels                                                           map[string]string
	createdAt                                                              time.Time
	disabledCount                                                          int
}

func versionCell(s Summary) g.Node {
	if s.DeploymentID != "" {
		return h.A(h.Href("/deployments/"+s.DeploymentID), g.Text(s.Version))
	}

	return g.Text(s.Version)
}

func statusCell(s Summary) g.Node {
	if s.Status == "DEPLOYED" && s.disabledCount > 0 {
		return g.Group([]g.Node{
			h.Span(h.Class("status-success"), g.Attr("title", fmt.Sprintf("Disabled in %d environment(s)", s.disabledCount)), g.Attr("style", "font-size:inherit;line-height:inherit"), g.Text("⚠️")),
			g.Text(" DEPLOYED"),
		})
	}
	return rolloutStatus(s.Status)
}

func rolloutStatus(status string) g.Node {
	switch strings.ToUpper(status) {
	case "DEPLOYED":
		return g.Group([]g.Node{h.Span(h.Class("status-success"), g.Text("✓")), g.Text(" DEPLOYED")})
	case "FAILED":
		return g.Group([]g.Node{h.Span(h.Class("status-error"), g.Text("✗")), g.Text(" FAILED")})
	case "PENDING":
		return g.Group([]g.Node{h.Span(h.Class("status-pending"), g.Text("⏳")), g.Text(" PENDING")})
	case "DISABLED":
		return g.Group([]g.Node{g.Text("⏸️ DISABLED")})
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
