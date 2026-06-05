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

		ch := make(chan reconciler.ComputeResult, 2048)

		// Forward decisions from channel to SSE as they arrive.
		total := 0
		var forwarder sync.WaitGroup
		forwarder.Add(1)
		go func() {
			defer forwarder.Done()
			for d := range ch {
				total++
				evt := sseDecision{
					Action:      d.Action.String(),
					Tenant:      d.TenantName,
					Environment: d.EnvironmentName,
					Feature:     d.Feature.Name,
					Version:     d.Feature.Version,
					Message:     d.Message,
				}
				b, _ := json.Marshal(evt)
				_, _ = w.Write(sseEvent("decision", b))
				flusher.Flush()
			}
		}()

		summary, err := rec.StreamDecisions(r.Context(), ch)
		forwarder.Wait() // ensure all decisions are written before continuing

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

		b, _ := json.Marshal(struct {
			FetchDur   int64 `json:"fetchDur"`
			ComputeDur int64 `json:"computeDur"`
			Total      int   `json:"total"`
		}{
			FetchDur:   summary.FetchDur.Nanoseconds(),
			ComputeDur: summary.ComputeDur.Nanoseconds(),
			Total:      total,
		})
		_, _ = w.Write(sseEvent("summary", b)) // #nosec G705 -- JSON-encoded internal data, not user input
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
