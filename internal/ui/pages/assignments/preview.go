package assignments

import (
	"encoding/json"
	"net/http"

	"github.com/nais/fasit/internal/environment"
)

type previewRequest struct {
	Labels map[string]string             `json:"labels"`
	Kinds  []environment.EnvironmentKind `json:"kinds"`
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

		tenantEnvs, err := environment.ListTenantEnvironments(r.Context(), false)
		if err != nil {
			http.Error(w, "failed to load environments", http.StatusInternalServerError)
			return
		}

		var matched []previewEnvironment
		for _, te := range tenantEnvs {
			if matchesKind(te.Kind, req.Kinds) && matchesLabels(te.Labels, req.Labels) {
				matched = append(matched, previewEnvironment{
					Tenant:      te.TenantName,
					Environment: te.Name,
					Labels:      te.Labels,
				})
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(matched); err != nil {
			http.Error(w, "Failed to encode response: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func matchesKind(kind environment.EnvironmentKind, kinds []environment.EnvironmentKind) bool {
	if len(kinds) == 0 {
		return true
	}
	for _, candidate := range kinds {
		if candidate == kind {
			return true
		}
	}
	return false
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
