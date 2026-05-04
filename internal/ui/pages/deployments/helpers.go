package deployments

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nais/fasit/internal/ui/layout"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, layout.Props)

type Summary struct {
	FeatureName, Version, Status, Target, Created, Completed, DeploymentID string
	createdAt                                                              time.Time
	disabledCount                                                          int
}

func versionCell(s Summary) g.Node {
	if s.DeploymentID != "" {
		return h.A(h.Href("/ui/deployments/"+s.DeploymentID), g.Text(s.Version))
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

var oslo = mustLoadLocation("Europe/Oslo")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}

	return loc
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	return t.In(oslo).Format("2006-01-02 15:04:05")
}
