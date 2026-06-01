package environment

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/nais/fasit/internal/ui/chart"
	"github.com/nais/fasit/internal/ui/components"
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

func FeatureContextPlaygroundTabHandler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := loadFeaturePageData(r.Context(), chi.URLParam(r, "tenant"), chi.URLParam(r, "env"), chi.URLParam(r, "feature"), "playground", "")
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

		renderPlaygroundPage(w, r, renderPage, data)
	}
}

func FeatureContextPlaygroundSubmitHandler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		data, err := loadFeaturePageData(r.Context(), chi.URLParam(r, "tenant"), chi.URLParam(r, "env"), chi.URLParam(r, "feature"), "playground", "")
		if err != nil {
			http.Error(w, "Failed to load data: "+err.Error(), http.StatusInternalServerError)
			return
		}

		code := r.FormValue("code")
		data.PlaygroundCode = code
		data.PlaygroundResult, _ = runPlayground(r.Context(), chi.URLParam(r, "tenant"), chi.URLParam(r, "env"), chi.URLParam(r, "feature"), code, r.FormValue("includeUnset") == "on")

		renderPlaygroundPage(w, r, renderPage, data)
	}
}

func renderPlaygroundPage(w http.ResponseWriter, r *http.Request, renderPage RenderPage, data *FeaturePage) {
	renderPage(w, r, layout.Props{
		Title:       data.Feature.Name + " / " + data.Tenant.Name + " / " + data.Environment.Name,
		CurrentPage: components.PageFeatures,
		Content:     featurePageContent(data),
	})
}
