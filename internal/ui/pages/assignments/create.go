package assignments

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
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
		version := requestedVersion(r.Form)
		description := strings.TrimSpace(r.FormValue("description"))
		if chart == "" || version == "" {
			http.Error(w, "chart and version are required", http.StatusBadRequest)
			return
		}
		var assignmentDescription *string
		if description != "" {
			assignmentDescription = &description
		}

		target := environment.Labels{}
		for _, raw := range r.Form["target_label"] {
			if raw == "" {
				continue
			}
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

		id, err := featureassignment.Create(r.Context(), featureassignment.CreateFeatureAssignment{
			Chart:       chart,
			Version:     version,
			Description: assignmentDescription,
			Target:      target,
		})
		if err != nil {
			http.Error(w, "Failed to create assignment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		reconciler.TriggerReconcile()

		redirect := "/assignments"
		if ref := r.Referer(); ref != "" {
			if u, err := url.Parse(ref); err == nil && u.Host == r.Host {
				redirect = u.Path
				if detailURL := newAssignmentDetailURL(u.Path, id); detailURL != "" {
					redirect = detailURL
				}
			}
		}
		http.Redirect(w, r, redirect, http.StatusSeeOther) // #nosec G710 -- redirect is the Referer path, or derived from it, only when its host == r.Host, forcing a same-origin relative path
	}
}

// newAssignmentDetailURL returns the detail page URL for the newly created
// assignment when refererPath is an assignment detail page. Setting a version
// from a detail page supersedes that assignment, so returning there would land
// the user on the old version.
func newAssignmentDetailURL(refererPath string, id uuid.UUID) string {
	parts := strings.Split(strings.Trim(refererPath, "/"), "/")
	if len(parts) != 4 || parts[0] != "features" || parts[2] != "assignments" {
		return ""
	}
	if _, err := uuid.Parse(parts[3]); err != nil {
		return ""
	}
	return "/features/" + parts[1] + "/assignments/" + id.String()
}

func requestedVersion(form url.Values) string {
	version := strings.TrimSpace(form.Get("version"))
	if version == "__custom__" {
		return strings.TrimSpace(form.Get("version_custom"))
	}
	return version
}
