package deployments

import (
	"net/http"
	"net/url"

	"github.com/nais/fasit/internal/deployment"
)

func ReconcileHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deployment.TriggerReconcile(r.Context(), deployment.ReconcileTriggerEvent{})
		http.Redirect(w, r, safeRefererPath(r.Header.Get("Referer")), http.StatusSeeOther)
	}
}

func safeRefererPath(ref string) string {
	u, err := url.Parse(ref)
	if err != nil || u.Path == "" {
		return "/"
	}
	return u.Path
}
