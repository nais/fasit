package auditlog

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/ui/uidata"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// UninstallLogsFragmentHandler returns an HTML fragment with the naisd helm
// output captured for an uninstall, keyed by its deploy-instruction id (diid).
// Used by the lazy modal on the audit log.
func UninstallLogsFragmentHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		diid, err := uuid.Parse(chi.URLParam(r, "diid"))
		if err != nil {
			http.Error(w, "Invalid log id", http.StatusBadRequest)
			return
		}

		lines, err := uidata.GetLogLines(r.Context(), diid)
		if err != nil {
			http.Error(w, "Failed to load logs", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = uninstallLogsFragment(lines).Render(w)
	}
}

func uninstallLogsFragment(lines []*uidata.LogLine) g.Node {
	if len(lines) == 0 {
		return h.Div(h.Class("modal-body"),
			h.H3(g.Text("Uninstall log")),
			h.P(h.Class("text-muted"), g.Text("No log output recorded for this uninstall.")),
		)
	}

	rows := make([]g.Node, 0, len(lines))
	for _, line := range lines {
		rows = append(rows,
			h.Div(
				h.Span(h.Class("text-muted"), h.Title(view.FormatTime(line.Timestamp)), g.Text(view.FormatTime(line.Timestamp)+"  ")),
				g.Text(line.Message),
			),
		)
	}

	return h.Div(h.Class("modal-body"),
		h.H3(g.Text("Uninstall log")),
		h.Pre(h.Class("code-block"), g.Group(rows)),
	)
}
