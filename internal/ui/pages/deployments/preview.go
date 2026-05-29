package deployments

import (
	"encoding/json"
	"net/http"

	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/ui/uidata"
)

type previewRequest struct {
	Labels map[string]string `json:"labels"`
}

type previewEnvironment struct {
	Tenant      string            `json:"tenant"`
	Environment string            `json:"environment"`
	Labels      map[string]string `json:"labels"`
}

func PreviewTargetsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req previewRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		tenants, err := uidata.ListTenants(r.Context())
		if err != nil {
			http.Error(w, "failed to load tenants", http.StatusInternalServerError)
			return
		}

		var matched []previewEnvironment
		for _, tenant := range tenants {
			envs, err := environment.List(r.Context(), tenant.ID)
			if err != nil {
				continue
			}
			for _, env := range envs {
				labels, err := environment.GetLabels(r.Context(), env.ID)
				if err != nil {
					continue
				}
				if matchesLabels(labels, req.Labels) {
					matched = append(matched, previewEnvironment{
						Tenant:      tenant.Name,
						Environment: env.Name,
						Labels:      labels,
					})
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(matched); err != nil {
			http.Error(w, "Failed to encode response: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

// matchesLabels returns true if env labels contain all target labels.
func matchesLabels(envLabels, targetLabels map[string]string) bool {
	if len(targetLabels) == 0 {
		return true
	}
	for k, v := range targetLabels {
		if envLabels[k] != v {
			return false
		}
	}
	return true
}
