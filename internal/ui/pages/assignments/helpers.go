package assignments

import (
	"fmt"
	"net/http"
	"time"

	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, *http.Request, layout.Props)

type Summary struct {
	FeatureName, Chart, Version, Status, Target, Created, Completed, FeatureAssignmentID string
	TargetLabels                                                                         map[string]string
	Creator                                                                              string
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
