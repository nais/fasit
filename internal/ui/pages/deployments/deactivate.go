package deployments

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database/types"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/reconciler"
)

func DeactivateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			http.Error(w, "Failed to parse deployment ID", http.StatusInternalServerError)
			return
		}

		if err := deployment.Deactivate(r.Context(), id); err != nil {
			http.Error(w, "Failed to deactivate deployment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		reconciler.TriggerReconcile()

		http.Redirect(w, r, deactivateRedirect(r), http.StatusSeeOther)
	}
}

func DeactivateByFeatureAndTargetHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			http.Error(w, "Failed to parse deployment ID", http.StatusInternalServerError)
			return
		}

		dep, err := deployment.Get(r.Context(), id)
		if err != nil {
			http.Error(w, "Failed to load deployment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := deployment.DeactivateByFeatureAndTarget(r.Context(), dep.Feature.Name, types.EnvironmentLabels(dep.TargetLabels)); err != nil {
			http.Error(w, "Failed to deactivate deployments: "+err.Error(), http.StatusInternalServerError)
			return
		}

		reconciler.TriggerReconcile()

		http.Redirect(w, r, deactivateRedirect(r), http.StatusSeeOther)
	}
}

func deactivateRedirect(r *http.Request) string {
	if redirect := r.FormValue("redirect"); redirect != "" && strings.HasPrefix(redirect, "/") {
		return redirect
	}
	if ref := r.Header.Get("Referer"); ref != "" {
		if u, err := url.Parse(ref); err == nil && u.Host == r.Host && strings.HasPrefix(u.Path, "/features/") {
			return u.Path
		}
	}
	return "/deployments"
}
