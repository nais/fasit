package deployments

import (
	"net/http"

	"github.com/nais/fasit/internal/deployment"
)

func ReconcileHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deployment.TriggerReconcile(r.Context(), deployment.ReconcileTriggerEvent{})
		referer := r.Header.Get("Referer")
		if referer == "" {
			referer = "/"
		}
		http.Redirect(w, r, referer, http.StatusSeeOther)
	}
}
