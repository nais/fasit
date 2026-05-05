package deployments

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nais/fasit/internal/ui/layout"
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

func relativeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	if d < 0 {
		return formatTime(t)
	}
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return strconv.Itoa(int(d/time.Minute)) + "m ago"
	case d < 24*time.Hour:
		return strconv.Itoa(int(d/time.Hour)) + "h ago"
	case d < 48*time.Hour:
		return "yesterday"
	case d < 30*24*time.Hour:
		return strconv.Itoa(int(d/(24*time.Hour))) + "d ago"
	case d < 365*24*time.Hour:
		return strconv.Itoa(int(d/(30*24*time.Hour))) + "mo ago"
	default:
		return strconv.Itoa(int(d/(365*24*time.Hour))) + "y ago"
	}
}

func timeWithTitle(t time.Time) g.Node {
	if t.IsZero() {
		return g.Text("")
	}
	return h.Span(g.Attr("title", formatTime(t)), g.Text(relativeTime(t)))
}
