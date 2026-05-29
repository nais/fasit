package deployments

import (
	"net/http"
	"net/url"

	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/reconciler"
)

func ReconcileHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reconciler.TriggerReconcile()
		_ = audit.Create(r.Context(), audit.CreateParams{
			Action:      audit.ActionTriggered,
			Description: "full reconcile",
			ObjectType:  audit.ObjectTypeDeployment,
			ObjectID:    "all",
		})
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
