package assignments

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/internal/featureassignment"
)

func UpdateDescriptionHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			http.Error(w, "Invalid assignment ID", http.StatusBadRequest)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		featureName, err := featureassignment.UpdateDescription(r.Context(), id, r.PostForm.Get("description"))
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "Failed to update assignment description", http.StatusInternalServerError)
			return
		}

		listURL := "/features/" + featureName + "/assignments"
		redirect := listURL + "/" + id.String()
		if r.PostForm.Get("redirect") == listURL {
			redirect = listURL
		}
		http.Redirect(w, r, redirect, http.StatusSeeOther) // #nosec G710 -- relative path built from the stored feature name and parsed UUID
	}
}
