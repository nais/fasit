package deployments

import (
	"net/http"

	"github.com/nais/fasit/internal/database/types"
	"github.com/nais/fasit/internal/deployment"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func DeactivateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			http.Error(w, "Failed to parse deployment ID", http.StatusInternalServerError)
			return
		}

		dep, err := deployment.GetDeployment(r.Context(), id)
		if err != nil {
			http.Error(w, "Failed to load deployment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := deployment.DeactivateDeploymentTarget(r.Context(), dep.Feature.Name, types.EnvironmentLabels(dep.TargetLabels)); err != nil {
			http.Error(w, "Failed to deactivate deployment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		redirect := r.FormValue("redirect")
		if redirect == "" {
			redirect = "/"
		}
		http.Redirect(w, r, redirect, http.StatusSeeOther)
	}
}

func ActivateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			http.Error(w, "Failed to parse deployment ID", http.StatusInternalServerError)
			return
		}

		dep, err := deployment.GetDeployment(r.Context(), id)
		if err != nil {
			http.Error(w, "Failed to load deployment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := deployment.ActivateDeploymentTarget(r.Context(), dep.Feature.Name, types.EnvironmentLabels(dep.TargetLabels)); err != nil {
			http.Error(w, "Failed to activate deployment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		redirect := r.FormValue("redirect")
		if redirect == "" {
			redirect = "/"
		}
		http.Redirect(w, r, redirect, http.StatusSeeOther)
	}
}

func ActivateVersionHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			http.Error(w, "Failed to parse deployment ID", http.StatusInternalServerError)
			return
		}

		if err := deployment.ActivateDeploymentByID(r.Context(), id); err != nil {
			http.Error(w, "Failed to activate deployment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		redirect := r.FormValue("redirect")
		if redirect == "" {
			redirect = "/deployments/" + id.String()
		}
		http.Redirect(w, r, redirect, http.StatusSeeOther)
	}
}
