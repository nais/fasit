package environment

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
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

		allTenants, _ := envpkg.GetTenants(r.Context())
		tenantEnvs, _ := envpkg.List(r.Context(), tenant.ID)

		environment := &Environment{
			Environment: env,
			Metadata:    getEnvironmentMetadata(r.Context(), env),
		}

		renderPage(w, r, layout.Props{
			Title:        tenant.Name + " / " + env.Name,
			CurrentPage:  components.PageTenants,
			GCPProjectID: gcpProjectIDFromMetadata(environment.Metadata),
			Content: page([]breadcrumb.Crumb{
				breadcrumb.TenantWithSwitcher(tenant.Name, toTenantNavs(allTenants)),
				breadcrumb.EnvironmentWithSwitcher(tenant.Name, env.Name, toEnvironmentNavs(tenantEnvs)),
			}, tenant, environment, allFeatures),
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

func page(breadcrumbs []breadcrumb.Crumb, tenant *model.Tenant, environment *Environment, allFeatures []view.FeatureNav) g.Node {
	return h.Div(h.Class("container"),
		components.EnvironmentSidebar(tenant.Name, environment.Name, "", allFeatures),
		h.Main(h.Class("main-content"),
			components.Breadcrumbs(breadcrumbs),
			h.Div(h.Class("card"),
				h.Div(h.Class("card-body"),
					h.H1(
						h.Span(h.Class("icon"), g.Text(view.TenantIcon(tenant.Name))),
						g.Text(" "),
						g.Text(tenant.Name+" / "+environment.Name),
					),
					h.P(h.Class("text-muted"), g.Textf("%d features enabled", countEnabled(allFeatures))),
					h.H2(g.Text("Environment Metadata")),
					h.Table(h.Class("table"),
						h.TBody(g.Group(g.Map(environment.Metadata, func(item MetadataItem) g.Node {
							return h.Tr(
								h.Td(h.Class("th-like width-md"), g.Text(item.Key)),
								h.Td(metadataValue(item)),
							)
						}))),
					),
				),
			),
		),
	)
}
