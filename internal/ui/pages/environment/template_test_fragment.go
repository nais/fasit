package environment

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	envpkg "github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/graph/model"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// TemplateTestHandler returns the template tester modal fragment (GET) or
// renders a template and returns the result (POST).
func TemplateTestHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		featureName := chi.URLParam(r, "feature")
		key := chi.URLParam(r, "key")

		tenant, err := envpkg.GetTenantByName(ctx, chi.URLParam(r, "tenant"))
		if err != nil {
			http.Error(w, "Tenant not found", http.StatusNotFound)
			return
		}
		env, err := envpkg.GetByName(ctx, tenant.ID, chi.URLParam(r, "env"))
		if err != nil {
			http.Error(w, "Environment not found", http.StatusNotFound)
			return
		}
		feat, err := featureassignment.FeatureForEnvironment(ctx, env.ID, featureName)
		if err != nil {
			http.Error(w, "Feature not found", http.StatusNotFound)
			return
		}

		val, ok := feat.Values[key]
		if !ok || val.Computed == nil {
			http.Error(w, "Not a computed key", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}
			template := r.FormValue("template")
			result, renderErr := renderSingleTemplate(ctx, feat, env.ID, key, template)
			node := templateTestResultFragment(key, val, template, result, renderErr)
			_ = node.Render(w)
			return
		}

		// GET: return the modal with the current template pre-filled
		rendered, renderErr := renderSingleTemplate(ctx, feat, env.ID, key, val.Computed.Template)
		node := templateTestFragment(key, val, rendered, renderErr)
		_ = node.Render(w)
	}
}

func renderSingleTemplate(ctx context.Context, feat *model.Feature, envID uuid.UUID, key, template string) (string, error) {
	// Create a modified feature with the test template
	modifiedValues := make(model.Values, len(feat.Values))
	for k, v := range feat.Values {
		modifiedValues[k] = v
	}
	modifiedVal := modifiedValues[key]
	modifiedVal.Computed = &model.Computed{Template: template}
	modifiedValues[key] = modifiedVal

	modifiedFeat := *feat
	modifiedFeat.Values = modifiedValues

	rendered, _, _, err := featurepkg.HelmValuesWithSecretTaint(ctx, &modifiedFeat, envID)
	if err != nil {
		return "", err
	}

	if v, ok := lookupHelmValue(rendered, key); ok {
		return v, nil
	}
	return "", nil
}

func templateTestFragment(key string, val model.Value, currentResult string, renderErr error) g.Node {
	var resultNode g.Node
	if renderErr != nil {
		resultNode = h.Div(
			h.Label(g.Text("Result")),
			h.Pre(h.Class("code-block template-test-error"), g.Text(renderErr.Error())),
		)
	} else {
		resultNode = h.Div(
			h.Label(g.Text("Result")),
			h.Pre(h.Class("code-block"), g.Text(currentResult)),
		)
	}

	return templateTestModal(val.Computed.Template, resultNode)
}

func templateTestResultFragment(key string, val model.Value, template, result string, renderErr error) g.Node {
	var resultNode g.Node
	if renderErr != nil {
		resultNode = h.Div(
			h.Label(g.Text("Result")),
			h.Pre(h.Class("code-block template-test-error"), g.Text(renderErr.Error())),
		)
	} else {
		resultNode = h.Div(
			h.Label(g.Text("Result")),
			h.Pre(h.Class("code-block"), g.Text(result)),
		)
	}

	return templateTestModal(template, resultNode)
}

func templateTestModal(template string, resultNode g.Node) g.Node {
	title := "Test template"
	return h.Div(h.Class("modal-body template-test"),
		h.Div(h.Class("modal-title"), g.Text(title)),
		h.Form(h.Class("template-test-form"), g.Attr("data-template-test", ""),
			h.Div(h.Class("template-test-label-row"),
				h.Label(h.For("template-input"), g.Text("Template")),
				templateHelpPopover(),
			),
			h.Textarea(h.Name("template"), h.ID("template-input"), h.Rows("4"),
				g.Text(template),
			),
			h.Div(h.Class("template-test-actions"),
				h.Button(h.Type("submit"), g.Text("Render")),
			),
			h.Div(h.Class("template-test-result-section"), resultNode),
		),
	)
}

func templateHelpPopover() g.Node {
	return h.Div(h.Class("help-popover-wrap"),
		h.Button(h.Type("button"), h.Class("help-toggle"), g.Attr("data-help-toggle", ""), g.Text("?")),
		h.Div(h.Class("help-popover"),
			h.H4(g.Text("Available variables")),
			h.Dl(
				h.Dt(g.Text(".Env")), h.Dd(g.Text("Map of environment config values (name, labels, etc.)")),
				h.Dt(g.Text(".Tenant.Name")), h.Dd(g.Text("Name of the owning tenant")),
				h.Dt(g.Text(".Envs")), h.Dd(g.Text("List of all environments for the tenant")),
				h.Dt(g.Text(".Configs")), h.Dd(g.Text("Map of all configurable values (resolved)")),
				h.Dt(g.Text(".Kind")), h.Dd(g.Text("Environment kind (management, tenant, onprem, …)")),
				h.Dt(g.Text(".Management")), h.Dd(g.Text("Config values from the management environment")),
			),
			h.H4(g.Text("Functions")),
			h.Dl(
				h.Dt(g.Text("toJSON")), h.Dd(g.Text("Encode value as JSON")),
				h.Dt(g.Text("fromJSON")), h.Dd(g.Text("Decode JSON string")),
				h.Dt(g.Text("toYAML")), h.Dd(g.Text("Encode value as YAML")),
				h.Dt(g.Text("b64enc")), h.Dd(g.Text("Base64 encode")),
				h.Dt(g.Text("quote")), h.Dd(g.Text("Wrap in quotes")),
				h.Dt(g.Text("replace")), h.Dd(g.Text("replace s old new")),
				h.Dt(g.Text("join")), h.Dd(g.Text("join sep list")),
				h.Dt(g.Text("filter / exclude")), h.Dd(g.Text("Filter list of maps by key=value")),
				h.Dt(g.Text("mapOf")), h.Dd(g.Text("mapOf keyField valueField list")),
				h.Dt(g.Text("mapJoin")), h.Dd(g.Text("Join map values with separator")),
				h.Dt(g.Text("subdomain")), h.Dd(g.Text("Extract subdomain from hostname")),
				h.Dt(g.Text("prefixedValues")), h.Dd(g.Text("Get configs matching a prefix")),
				h.Dt(g.Text("eachOf")), h.Dd(g.Text("Iterate list, apply template")),
			),
			h.P(h.Class("text-muted text-sm"), g.Text("All "), h.A(h.Href("https://masterminds.github.io/sprig/"), g.Attr("target", "_blank"), g.Text("sprig functions")), g.Text(" are also available.")),
		),
	)
}
