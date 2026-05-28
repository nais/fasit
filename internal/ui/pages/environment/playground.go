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
	return playgroundTabHandler(renderPage, true)
}

func playgroundTabHandler(renderPage RenderPage, featureContext bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := loadFeaturePageData(r.Context(), chi.URLParam(r, "tenant"), chi.URLParam(r, "env"), chi.URLParam(r, "feature"), "playground", featureContext)
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

		renderPlaygroundPage(w, r, renderPage, data, featureContext)
	}
}

func PlaygroundSubmitHandler(renderPage RenderPage) http.HandlerFunc {
	return playgroundSubmitHandler(renderPage, false)
}

func FeatureContextPlaygroundSubmitHandler(renderPage RenderPage) http.HandlerFunc {
	return playgroundSubmitHandler(renderPage, true)
}

func playgroundSubmitHandler(renderPage RenderPage, featureContext bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		data, err := loadFeaturePageData(r.Context(), chi.URLParam(r, "tenant"), chi.URLParam(r, "env"), chi.URLParam(r, "feature"), "playground", featureContext)
		if err != nil {
			http.Error(w, "Failed to load data: "+err.Error(), http.StatusInternalServerError)
			return
		}

		code := r.FormValue("code")
		data.PlaygroundCode = code
		data.PlaygroundResult, _ = runPlayground(r.Context(), chi.URLParam(r, "tenant"), chi.URLParam(r, "env"), chi.URLParam(r, "feature"), code, r.FormValue("includeUnset") == "on")

		renderPlaygroundPage(w, r, renderPage, data, featureContext)
	}
}

func renderPlaygroundPage(w http.ResponseWriter, r *http.Request, renderPage RenderPage, data *FeaturePage, featureContext bool) {
	title := data.Tenant.Name + " / " + data.Environment.Name + " / " + data.Feature.Name
	currentPage := components.PageEnvironments
	if featureContext {
		title = data.Feature.Name + " / " + data.Tenant.Name + " / " + data.Environment.Name
		currentPage = components.PageFeatures
	}
	renderPage(w, r, layout.Props{
		Title:       title,
		CurrentPage: currentPage,
		Content:     featurePageContent(data),
	})
}
