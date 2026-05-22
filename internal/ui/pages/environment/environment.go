package environment

import (
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/nais/fasit/internal/deployment"
	envpkg "github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/ui/breadcrumb"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, *http.Request, layout.Props)

type Environment struct {
	*model.Environment
	Metadata []MetadataItem
}

type MetadataItem struct {
	Key          string
	Value        string
	IsSecret     bool
	ReferencedBy []string
}

func Handler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantSlug := chi.URLParam(r, "tenant")
		envName := chi.URLParam(r, "env")

		tenant, err := envpkg.GetTenantByName(r.Context(), tenantSlug)
		if err != nil {
			http.Error(w, "Failed to load environment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		env, err := envpkg.GetByName(r.Context(), tenant.ID, envName)
		if err != nil {
			http.Error(w, "Failed to load environment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		allFeatures, err := featureNavs(r.Context(), env)
		if err != nil {
			http.Error(w, "Failed to load environment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		allTenants, _ := envpkg.ListTenants(r.Context())
		tenantEnvs, _ := envpkg.List(r.Context(), tenant.ID)
		labels, _ := envpkg.GetLabels(r.Context(), env.ID)

		environment := &Environment{
			Environment: env,
			Metadata:    getEnvironmentMetadata(r.Context(), env),
		}
		envValues, _ := envpkg.ListEnvironmentValuesForEnvironment(r.Context(), env.ID, true)
		valueRefs, _ := deployment.ValueRefsForEnvironment(r.Context(), env.ID)

		renderPage(w, r, layout.Props{
			Title:       tenant.Name + " / " + env.Name,
			CurrentPage: components.PageEnvironments,
			Content: page([]breadcrumb.Crumb{
				tenantCrumb(tenant.Name, toTenantNavs(allTenants)),
				breadcrumb.EnvironmentWithSwitcher(tenant.Name, env.Name, toEnvironmentNavs(tenantEnvs)),
			}, tenant, environment, allFeatures, labels, envValues, valueRefs, gcpProjectIDFromValues(envValues)),
		})
	}
}

func metadataValue(item MetadataItem) g.Node {
	var value g.Node
	if item.IsSecret {
		value = h.Span(h.Class("text-muted"), g.Text("••••••••"))
	} else {
		value = g.Text(item.Value)
	}
	if item.ReferencedBy == nil {
		return value
	}
	count := len(item.ReferencedBy)
	tooltip := strings.Join(item.ReferencedBy, ", ")
	return g.Group([]g.Node{
		value,
		g.Text(" "),
		h.Span(h.Class("badge"), h.Title(tooltip), g.Textf("%d ref%s", count, plural(count))),
	})
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func page(breadcrumbs []breadcrumb.Crumb, tenant *model.Tenant, environment *Environment, allFeatures []view.FeatureNav, labels map[string]string, envValues []*model.EnvironmentValue, valueRefs map[string][]string, gcpProjectID string) g.Node {
	return h.Div(h.Class("container"),
		components.EnvironmentSidebar(tenant.Name, environment.Name, "", allFeatures),
		h.Main(h.Class("main-content"),
			components.Breadcrumbs(breadcrumbs),

			// Environment header: name, kind, labels, actions
			h.Div(h.Class("env-header"),
				h.Div(h.Class("env-header-top"),
					h.H1(h.Class("env-title"), g.Text(environment.Name)),
				),
				h.Div(h.Class("env-meta-row"),
					g.Group(g.Map(environment.Metadata, func(item MetadataItem) g.Node {
						return h.Span(h.Class("env-meta-item"),
							h.Span(h.Class("env-meta-label"), g.Text(item.Key)),
							h.Span(h.Class("env-meta-value"), metadataValue(item)),
						)
					})),
					g.If(gcpProjectID != "", h.Span(h.Class("env-meta-item"),
						h.Span(h.Class("env-meta-label"), g.Text("GCP Project")),
						h.Span(h.Class("env-meta-value"),
							g.Text(gcpProjectID),
							g.Text(" "),
							h.A(
								h.Href("https://console.cloud.google.com/welcome?project="+gcpProjectID),
								g.Attr("target", "_blank"),
								g.Attr("rel", "noopener noreferrer"),
								g.Attr("title", "Open GCP project "+gcpProjectID),
								components.ExternalLinkIcon(),
							),
						),
					)),
				),
				g.If(len(labels) > 0, h.Div(h.Class("label-pills env-labels"),
					g.Group(g.Map(sortedKeys(labels), func(k string) g.Node {
						return h.Span(h.Class("label-filter-tag"), g.Textf("%s: %s", k, labels[k]))
					})),
				)),
			),

			// Environment values
			g.If(len(envValues) > 0, h.Div(h.Class("card"),
				h.Div(h.Class("card-body"),
					h.H2(h.Style("margin-top:0"), g.Text("Environment Values")),
					h.Table(h.Class("table"),
						h.TBody(g.Group(g.Map(envValues, func(val *model.EnvironmentValue) g.Node {
							var valNode g.Node
							if val.Secret {
								valNode = h.Span(h.Class("text-muted"), g.Text("••••••••"))
							} else {
								valNode = g.Text(components.RawValueForDisplay(val.Value))
							}
							if refs := valueRefs[val.Key]; len(refs) > 0 {
								tooltip := strings.Join(refs, ", ")
								valNode = g.Group([]g.Node{
									valNode,
									g.Text(" "),
									h.Span(h.Class("badge"), h.Title(tooltip), g.Textf("%d ref%s", len(refs), plural(len(refs)))),
								})
							}
							return h.Tr(
								h.Td(h.Class("th-like width-md"), g.Text(val.Key)),
								h.Td(valNode),
							)
						}))),
					),
				),
			)),
		),
	)
}
