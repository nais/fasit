package deployments

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database/types"
	"github.com/nais/fasit/internal/deployment"
)

func DeleteHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			http.Error(w, "Failed to parse deployment ID", http.StatusInternalServerError)
			return
		}

		if err := deployment.DeleteDeployment(r.Context(), id); err != nil {
			http.Error(w, "Failed to delete deployment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/ui/deployments", http.StatusSeeOther)
	}
}

func DeleteByFeatureAndTargetHandler() http.HandlerFunc {
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

		if err := deployment.DeleteDeploymentsByFeatureAndTarget(r.Context(), dep.Feature.Name, types.EnvironmentLabels(dep.TargetLabels), dep.CI); err != nil {
			http.Error(w, "Failed to delete deployments: "+err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/ui/deployments", http.StatusSeeOther)
	}
}
