package environment

import (
	"net/http"

	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/ui/chart"
	"github.com/nais/fasit/internal/ui/layout"
)

var defaultPlaygroundCode = `environmentKinds:
  - tenant
  - management
values:
  ingress.host:
    displayName: Ingress
    description: The host name of the ingress
    computed:
      template: |
        {{subdomain . "my-app"}}
`

func PlaygroundTabHandler(renderPage RenderPage, repo database.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := loadFeaturePageData(r.Context(), repo, r.PathValue("tenant"), r.PathValue("env"), r.PathValue("feature"), "playground")
		if err != nil {
			http.Error(w, "Failed to load data: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if yaml, err := chart.FetchFeatureYAML(data.Feature.Chart, data.Feature.Version); err == nil {
			data.PlaygroundCode = yaml
		}
		if data.PlaygroundCode == "" {
			data.PlaygroundCode = defaultPlaygroundCode
		}

		renderPage(w, layout.Props{
			Title:          data.Tenant.Name + " / " + data.Environment.Name + " / " + data.Feature.Name,
			CurrentSection: "tenants",
			Content:        featurePageContent(data),
		})
	}
}

func PlaygroundSubmitHandler(renderPage RenderPage, repo database.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		data, err := loadFeaturePageData(r.Context(), repo, r.PathValue("tenant"), r.PathValue("env"), r.PathValue("feature"), "playground")
		if err != nil {
			http.Error(w, "Failed to load data: "+err.Error(), http.StatusInternalServerError)
			return
		}

		code := r.FormValue("code")
		data.PlaygroundCode = code
		data.PlaygroundResult, _ = runPlayground(r.Context(), repo, r.PathValue("tenant"), r.PathValue("env"), r.PathValue("feature"), code, r.FormValue("includeUnset") == "on")

		renderPage(w, layout.Props{
			Title:          data.Tenant.Name + " / " + data.Environment.Name + " / " + data.Feature.Name,
			CurrentSection: "tenants",
			Content:        featurePageContent(data),
		})
	}
}
