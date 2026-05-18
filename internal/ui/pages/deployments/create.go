package deployments

import (
	"net/http"
	"strings"

	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/environment"
)

func CreateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		featureName := strings.TrimSpace(r.FormValue("feature_name"))
		chart := strings.TrimSpace(r.FormValue("chart"))
		version := strings.TrimSpace(r.FormValue("version"))
		if featureName == "" || chart == "" || version == "" {
			http.Error(w, "feature_name, chart, and version are required", http.StatusBadRequest)
			return
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

		_, err := deployment.CreateDeployment(r.Context(), deployment.Request{
			Chart:       chart,
			Version:     version,
			Description: "Set via UI",
			Target:      target,
		})
		if err != nil {
			http.Error(w, "Failed to create deployment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		deployment.TriggerReconcile(r.Context(), deployment.ReconcileTriggerEvent{})

		redirect := r.Referer()
		if redirect == "" {
			redirect = "/features/" + featureName
		}
		http.Redirect(w, r, redirect, http.StatusSeeOther)
	}
}
