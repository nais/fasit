package assignments

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/reconciler"
)

func CreateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		chart := strings.TrimSpace(r.FormValue("chart"))
		version := strings.TrimSpace(r.FormValue("version"))
		description := strings.TrimSpace(r.FormValue("description"))
		if chart == "" || version == "" {
			http.Error(w, "chart and version are required", http.StatusBadRequest)
			return
		}
		if description == "" {
			description = "Set via UI"
		}

		target := environment.Labels{}
		for _, raw := range r.Form["target_label"] {
			k, v, ok := strings.Cut(raw, "=")
			if !ok || k == "" {
				http.Error(w, "invalid target_label: "+raw, http.StatusBadRequest)
				return
			}
			target[k] = v
		}
		if raw := strings.TrimSpace(r.FormValue("target_labels_raw")); raw != "" {
			// Try JSON first, fall back to key=value lines
			if err := json.Unmarshal([]byte(raw), &target); err != nil {
				for _, line := range strings.Split(raw, "\n") {
					line = strings.TrimSpace(line)
					if line == "" {
						continue
					}
					k, v, ok := strings.Cut(line, "=")
					if !ok || k == "" {
						http.Error(w, "invalid target label: "+line, http.StatusBadRequest)
						return
					}
					target[strings.TrimSpace(k)] = strings.TrimSpace(v)
				}
			}
		}

		_, err := featureassignment.Create(r.Context(), featureassignment.CreateFeatureAssignment{
			Chart:       chart,
			Version:     version,
			Description: &description,
			Target:      target,
		})
		if err != nil {
			http.Error(w, "Failed to create assignment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		reconciler.TriggerReconcile()

		redirect := r.Referer()
		if redirect == "" {
			redirect = "/assignments"
		}
		http.Redirect(w, r, redirect, http.StatusSeeOther)
	}
}
