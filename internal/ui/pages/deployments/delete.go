package deployments

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/audit"
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

		if err := deployment.Delete(r.Context(), id); err != nil {
			http.Error(w, "Failed to delete deployment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		_ = audit.Create(r.Context(), audit.CreateParams{
			Description: fmt.Sprintf("deleted deployment %s", id),
			ObjectType:  "deployment",
			ObjectID:    id.String(),
		})

		http.Redirect(w, r, "/deployments", http.StatusSeeOther)
	}
}

func DeleteByFeatureAndTargetHandler() http.HandlerFunc {
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

		if err := deployment.DeleteDeploymentsByFeatureAndTarget(r.Context(), dep.Feature.Name, types.EnvironmentLabels(dep.TargetLabels)); err != nil {
			http.Error(w, "Failed to delete deployments: "+err.Error(), http.StatusInternalServerError)
			return
		}

		_ = audit.Create(r.Context(), audit.CreateParams{
			Description: fmt.Sprintf("deleted all deployments for %s", dep.Feature.Name),
			ObjectType:  "deployment",
			ObjectID:    dep.Feature.Name,
		})

		http.Redirect(w, r, "/deployments", http.StatusSeeOther)
	}
}
