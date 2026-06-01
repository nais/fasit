package assignments

import (
	"fmt"
	"net/http"
	"time"

	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, *http.Request, layout.Props)

type Summary struct {
	FeatureName, Chart, Version, Status, Target, Created, Completed, FeatureAssignmentID string
	TargetLabels                                                                         map[string]string
	Active                                                                               bool
	createdAt                                                                            time.Time
	disabledCount                                                                        int
}

func statusCell(s Summary) g.Node {
	if s.Status == "DEPLOYED" && s.disabledCount > 0 {
		return g.Group([]g.Node{
			h.Span(h.Class("status-success status-icon-inline"), g.Attr("title", fmt.Sprintf("Disabled in %d environment(s)", s.disabledCount)), g.Text("⚠️")),
			g.Text(" Deployed"),
		})
	}
	return components.Status(s.Status)
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
