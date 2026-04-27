package environment

import (
	"fmt"
	"net/http"

	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/ui/chart"
	"github.com/nais/fasit/internal/ui/layout"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
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
		if err != nil { http.Error(w, "Failed to load data: "+err.Error(), http.StatusInternalServerError); return }
		if yaml, err := chart.FetchFeatureYAML(data.Feature.Chart, data.Feature.Version); err == nil { data.PlaygroundCode = yaml }
		if data.PlaygroundCode == "" { data.PlaygroundCode = defaultPlaygroundCode }
		renderPage(w, layout.Props{Title: data.Tenant.Name + " / " + data.Environment.Name + " / " + data.Feature.Name, CurrentSection: "tenants", Content: featurePageContent(data)})
	}
}

func PlaygroundSubmitHandler(renderPage RenderPage, repo database.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil { http.Error(w, "Bad request", http.StatusBadRequest); return }
		data, err := loadFeaturePageData(r.Context(), repo, r.PathValue("tenant"), r.PathValue("env"), r.PathValue("feature"), "playground")
		if err != nil { http.Error(w, "Failed to load data: "+err.Error(), http.StatusInternalServerError); return }
		code := r.FormValue("code")
		data.PlaygroundCode = code
		data.PlaygroundResult, _ = runPlayground(r.Context(), repo, r.PathValue("tenant"), r.PathValue("env"), r.PathValue("feature"), code, r.FormValue("includeUnset") == "on")
		renderPage(w, layout.Props{Title: data.Tenant.Name + " / " + data.Environment.Name + " / " + data.Feature.Name, CurrentSection: "tenants", Content: featurePageContent(data)})
	}
}

func playgroundTab(page *FeaturePage) g.Node {
	code := page.PlaygroundCode
	if code == "" { code = defaultPlaygroundCode }
	action := fmt.Sprintf("/ui/tenants/%s/envs/%s/%s/playground", page.TenantSlug, page.Environment.Name, page.Feature.Name)
	return h.Div(h.Class("tab-content-wrapper"), h.Form(h.Method("POST"), h.Action(action), h.Div(h.Class("playground-controls"), h.Label(h.Input(h.Type("checkbox"), h.Name("includeUnset")), g.Text(" Include unset config")), h.Button(h.Type("submit"), g.Text("Generate"))), h.Div(h.Class("playground-split"), h.Div(h.Class("playground-editor"), h.Label(h.For("code"), g.Text("Feature.yaml")), h.Textarea(h.Name("code"), h.ID("pg-code"), g.Text(code))), h.Div(h.Class("playground-result"), playgroundResultNode(page.PlaygroundResult, page.HelmValues)))))
}

func playgroundResultNode(result *PlaygroundResult, helmValues string) g.Node {
	if result == nil { if helmValues == "" { return nil }; return h.Div(h.Class("playground-output"), h.H2(g.Text("values.yaml")), h.Pre(h.Class("code-block"), g.Text(prettyJSON(helmValues)))) }
	nodes := []g.Node{}
	if len(result.Errors) > 0 { errs := make([]g.Node, 0, len(result.Errors)); for _, e := range result.Errors { errs = append(errs, h.Li(g.Text(e))) }; nodes = append(nodes, h.Div(h.Class("playground-errors"), h.H2(g.Text("Errors")), h.Ul(errs...))) }
	if result.Result != "" { nodes = append(nodes, h.Div(h.Class("playground-output"), h.H2(g.Text("values.yaml")), h.Pre(h.Class("code-block"), g.Text(result.Result)))) }
	return g.Group(nodes)
}
