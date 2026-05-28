package reconcilerpage

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync"

	"github.com/nais/fasit/internal/reconciler"
)

type sseDecision struct {
	Action      string `json:"action"`
	Tenant      string `json:"tenant"`
	Environment string `json:"environment"`
	Feature     string `json:"feature"`
	Version     string `json:"version"`
	Message     string `json:"message"`
}

func StreamHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec := reconciler.FromContext(r.Context())
		if rec == nil {
			http.Error(w, "reconciler not available", http.StatusInternalServerError)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		// Serialize SSE writes — emit is called from parallel goroutines
		// but http.ResponseWriter is not goroutine-safe.
		var mu sync.Mutex

		summary, err := rec.StreamDecisions(r.Context(), func(d reconciler.DeployDecision) {
			evt := sseDecision{
				Action:      d.Action.String(),
				Tenant:      d.TenantName,
				Environment: d.EnvironmentName,
				Feature:     d.Feature.Name,
				Version:     d.Feature.Version,
				Message:     d.Message,
			}
			b, _ := json.Marshal(evt)

			mu.Lock()
			_, _ = w.Write(sseEvent("decision", b))
			flusher.Flush()
			mu.Unlock()
		})

		if err != nil {
			if errors.Is(err, reconciler.ErrReconcileInProgress) {
				_, _ = w.Write(sseEvent("error", []byte(`{"error":"reconcile already in progress"}`)))
				flusher.Flush()
				return
			}
			errJSON, _ := json.Marshal(map[string]string{"error": err.Error()})
			_, _ = w.Write(sseEvent("error", errJSON))
			flusher.Flush()
			return
		}

		b, _ := json.Marshal(summary)
		_, _ = w.Write(sseEvent("summary", b))
		flusher.Flush()
	}
}

func sseEvent(event string, data []byte) []byte {
	buf := make([]byte, 0, len("event: ")+len(event)+len("\ndata: ")+len(data)+len("\n\n"))
	buf = append(buf, "event: "...)
	buf = append(buf, event...)
	buf = append(buf, "\ndata: "...)
	buf = append(buf, data...)
	buf = append(buf, "\n\n"...)
	return buf
}
