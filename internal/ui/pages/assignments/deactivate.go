package assignments

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database/types"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/reconciler"
)

func DeactivateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			http.Error(w, "Failed to parse assignment ID", http.StatusInternalServerError)
			return
		}

		if err := featureassignment.Deactivate(r.Context(), id); err != nil {
			http.Error(w, "Failed to deactivate assignment: "+err.Error(), http.StatusInternalServerError)
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
			http.Error(w, "Failed to parse assignment ID", http.StatusInternalServerError)
			return
		}

		fa, err := featureassignment.Get(r.Context(), id)
		if err != nil {
			http.Error(w, "Failed to load assignment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := featureassignment.DeactivateByFeatureAndTarget(r.Context(), fa.Feature.Name, types.EnvironmentLabels(fa.TargetLabels)); err != nil {
			http.Error(w, "Failed to deactivate assignments: "+err.Error(), http.StatusInternalServerError)
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
	return "/assignments"
}
