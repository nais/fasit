package templatetest

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	envpkg "github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/feature/featureutil"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, *http.Request, layout.Props)

func Handler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		tenantEnvs, err := envpkg.ListTenantEnvironments(ctx, false)
		if err != nil {
			http.Error(w, "Failed to load environments", http.StatusInternalServerError)
			return
		}

		var result string
		var renderErr error

		template := r.FormValue("template")
		featureName := r.FormValue("feature")
		key := r.FormValue("key")

		// Resolve tenant/env from the combined select value or separate params
		var tenantName, envName string
		if combined := r.FormValue("env"); combined != "" {
			if parts := strings.SplitN(combined, "/", 2); len(parts) == 2 {
				tenantName, envName = parts[0], parts[1]
			}
		}
		if tenantName == "" {
			tenantName = r.FormValue("tenant")
			envName = r.FormValue("env")
		}

		var envID uuid.UUID
		if tenantName != "" && envName != "" {
			if tenant, err := envpkg.GetTenantByName(ctx, tenantName); err == nil {
				if env, err := envpkg.GetByName(ctx, tenant.ID, envName); err == nil {
					envID = env.ID
				}
			}
		}

		// Pre-fill template from feature's computed key if not explicitly provided
		if template == "" && featureName != "" && key != "" && envID != uuid.Nil {
			feat, err := featureassignment.FeatureForEnvironment(ctx, envID, featureName)
			if err == nil {
				if val, ok := feat.Values[key]; ok && val.Computed != nil {
					template = val.Computed.Template
				}
			}
		}

		// Render if we have both env and template
		if envID != uuid.Nil && template != "" {
			if featureName != "" && key != "" {
				result, renderErr = renderWithFeature(ctx, featureName, envID, key, template)
			} else {
				mv, _, mvErr := featurepkg.MappingValuesForEnvironment(ctx, envID, false)
				if mvErr != nil {
					renderErr = mvErr
				} else {
					result, renderErr = featurepkg.RenderSingleTemplate(mv, template)
				}
			}
		}

		renderPage(w, r, layout.Props{
			Title:       "Template tester",
			CurrentPage: components.PageTemplateTester,
			Content:     templateTestPage(tenantEnvs, template, tenantName, envName, featureName, key, result, renderErr),
		})
	}
}

func renderWithFeature(ctx context.Context, featureName string, envID uuid.UUID, key, template string) (string, error) {
	feat, err := featureassignment.FeatureForEnvironment(ctx, envID, featureName)
	if err != nil {
		return "", err
	}

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

func lookupHelmValue(m map[string]any, key string) (string, bool) {
	keys, err := featureutil.SmartDotSplit(key)
	if err != nil || len(keys) == 0 {
		return "", false
	}
	var cur any = m
	for _, k := range keys {
		mm, ok := cur.(map[string]any)
		if !ok {
			return "", false
		}
		cur, ok = mm[k]
		if !ok {
			return "", false
		}
	}
	switch v := cur.(type) {
	case string:
		return v, true
	case nil:
		return "", true
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", false
		}
		return string(b), true
	}
}

func templateTestPage(tenantEnvs []*model.TenantEnvironment, template, selectedTenant, selectedEnv, featureName, key, result string, renderErr error) g.Node {
	envOpts := make([]g.Node, 0, len(tenantEnvs)+1)
	envOpts = append(envOpts, h.Option(h.Value(""), g.Text("Select environment…")))
	for _, te := range tenantEnvs {
		val := te.TenantName + "/" + te.Name
		attrs := []g.Node{h.Value(val), g.Text(te.TenantName + " / " + te.Name)}
		if te.TenantName == selectedTenant && te.Name == selectedEnv {
			attrs = append(attrs, g.Attr("selected", "selected"))
		}
		envOpts = append(envOpts, h.Option(attrs...))
	}

	var resultNode g.Node
	if renderErr != nil {
		resultNode = h.Pre(h.Class("code-block template-test-error"), g.Text(renderErr.Error()))
	} else if result != "" {
		resultNode = h.Pre(h.Class("code-block"), g.Text(result))
	} else {
		resultNode = h.Pre(h.Class("code-block template-test-empty"), g.Text("Result will appear here"))
	}

	// Hidden fields to preserve feature/key context across form submissions
	var hiddenFields g.Node
	if featureName != "" {
		hiddenFields = g.Group([]g.Node{
			h.Input(h.Type("hidden"), h.Name("feature"), h.Value(featureName)),
			h.Input(h.Type("hidden"), h.Name("key"), h.Value(key)),
		})
	}

	return h.Div(h.Class("container"),
		h.Main(h.Class("main-content"),
			h.Div(h.Class("template-test-layout"),
				h.Div(h.Class("template-test-main"),
					components.Card(
						h.H2(g.Text("Template tester")),
						h.Form(h.Method("GET"), h.Class("template-test-page-form"),
							hiddenFields,
							h.Div(h.Class("form-group"),
								h.Label(h.For("env-select"), g.Text("Environment")),
								h.Select(h.Name("env"), h.ID("env-select"), g.Group(envOpts)),
							),
							h.Div(h.Class("form-group"),
								h.Label(h.For("template-input"), g.Text("Template")),
								h.Textarea(h.Name("template"), h.ID("template-input"), h.Rows("6"),
									g.Text(template),
								),
							),
							h.Div(h.Class("template-test-actions"),
								h.Button(h.Type("submit"), h.Class("btn-secondary"), g.Text("Render")),
								h.Span(h.Class("text-muted text-sm"), g.Text("Shift+Enter")),
							),
							h.Div(h.Class("form-group"),
								h.Label(g.Text("Result")),
								resultNode,
							),
						),
					),
				),
				templateReference(),
			),
		),
	)
}

func templateReference() g.Node {
	type refItem struct {
		Name    string
		Example string
	}

	variables := []refItem{
		{".Env", ""},
		{".Env.name", ""},
		{".Env.labels", ""},
		{".Tenant.Name", ""},
		{".Envs", ""},
		{".Configs", ""},
		{".Kind", ""},
		{".Management", ""},
	}

	customFuncs := []refItem{
		{"toJSON", `{{ .Env | toJSON }}`},
		{"fromJSON", `{{ fromJSON .Configs.raw }}`},
		{"toYAML", `{{ .Env | toYAML }}`},
		{"b64enc", `{{ "secret" | b64enc }} → c2VjcmV0`},
		{"quote", `{{ "hello" | quote }} → "hello"`},
		{"replace", `{{ "a.b.c" | replace "." "_" }} → a_b_c`},
		{"join", `{{ join "," (list "a" "b") }} → a,b`},
		{"filter", `{{ filter .Envs "kind" "tenant" }}`},
		{"exclude", `{{ exclude .Envs "kind" "management" }}`},
		{"mapOf", `{{ mapOf "name" "url" .Envs }}`},
		{"mapJoin", `{{ mapJoin "," .myMap }}`},
		{"subdomain", `{{ subdomain "app.example.com" }} → app`},
		{"prefixedValues", `{{ prefixedValues .Configs "db." }}`},
		{"eachOf", `{{ eachOf .Envs "tmpl" }}`},
	}

	sprigFuncs := []refItem{
		{"default", `{{ .Configs.port | default "8080" }} → 8080`},
		{"ternary", `{{ ternary "yes" "no" true }} → yes`},
		{"upper / lower", `{{ "dev" | upper }} → DEV`},
		{"trimSuffix", `{{ trimSuffix ".svc" "app.svc" }} → app`},
		{"trimPrefix", `{{ trimPrefix "https://" "https://x" }} → x`},
		{"contains", `{{ if contains "prod" .Env.name }}...{{ end }}`},
		{"hasPrefix", `{{ if hasPrefix "dev" .Env.name }}...{{ end }}`},
		{"printf", `{{ printf "%s-%s" "a" "b" }} → a-b`},
		{"list", `{{ list "a" "b" "c" }}`},
		{"dict", `{{ dict "key" "val" | toJSON }}`},
		{"hasKey", `{{ if hasKey .Env "labels" }}...{{ end }}`},
		{"indent", `{{ "data" | indent 4 }}`},
	}

	renderItems := func(items []refItem) g.Node {
		nodes := make([]g.Node, 0, len(items))
		for _, item := range items {
			attrs := []g.Node{h.Class("ref-item"), g.Text(item.Name)}
			if item.Example != "" {
				attrs = []g.Node{h.Class("ref-item"), g.Attr("data-tip", item.Example), g.Text(item.Name)}
			}
			nodes = append(nodes, h.Span(attrs...))
		}
		return g.Group(nodes)
	}

	return h.Aside(h.Class("template-test-reference"),
		h.H3(g.Text("Reference")),
		h.H4(g.Text("Variables")),
		h.Div(h.Class("ref-list"), renderItems(variables)),
		h.H4(g.Text("Custom functions")),
		h.Div(h.Class("ref-list"), renderItems(customFuncs)),
		h.H4(g.Text("Sprig functions")),
		h.Div(h.Class("ref-list"), renderItems(sprigFuncs)),
		h.P(h.Class("text-muted text-sm"), g.Text("Full list: "), h.A(h.Href("https://masterminds.github.io/sprig/"), g.Attr("target", "_blank"), g.Text("sprig docs"))),
	)
}
