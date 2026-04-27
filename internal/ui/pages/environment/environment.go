package environment

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/nais/fasit/internal/database"
	envpkg "github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/ui/breadcrumb"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, layout.Props)

type Environment struct {
	*model.Environment
	Metadata []MetadataItem
}

type MetadataItem struct {
	Key   string
	Value string
}

func Handler(renderPage RenderPage, repo database.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantSlug := chi.URLParam(r, "tenant")
		envName := chi.URLParam(r, "env")

		tenant, err := envpkg.GetTenantGetByName(r.Context(), tenantSlug)
		if err != nil {
			http.Error(w, "Failed to load environment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		env, err := repo.EnvironmentGetByName(r.Context(), tenant.ID, envName)
		if err != nil {
			http.Error(w, "Failed to load environment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		allFeatures, enabledFeatures, err := featureNavs(r.Context(), env)
		if err != nil {
			http.Error(w, "Failed to load environment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		allTenants, _ := envpkg.GetTenants(r.Context())
		tenantEnvs, _ := repo.EnvironmentsGet(r.Context(), tenant.ID)

		environment := &Environment{
			Environment: env,
			Metadata:    getEnvironmentMetadata(r.Context(), repo, env),
		}

		renderPage(w, layout.Props{
			Title:          tenant.Name + " / " + env.Name,
			CurrentSection: "tenants",
			Content: page([]breadcrumb.Crumb{
				breadcrumb.TenantWithSwitcher(tenant.Name, toTenantNavs(allTenants)),
				breadcrumb.EnvironmentWithSwitcher(tenant.Name, env.Name, toEnvironmentNavs(tenantEnvs)),
			}, tenant, environment, allFeatures, enabledFeatures),
		})
	}
}

func page(breadcrumbs []breadcrumb.Crumb, tenant *model.Tenant, environment *Environment, allFeatures, enabledFeatures []view.FeatureNav) g.Node {
	return h.Div(h.Class("container"),
		components.EnvironmentSidebar(tenant.Name, environment.Name, "", allFeatures, enabledFeatures),
		h.Main(h.Class("main-content"),
			components.Breadcrumbs(breadcrumbs),
			h.Div(h.Class("card"),
				h.Div(h.Class("card-body"),
					h.H1(
						h.Span(h.Class("icon"), g.Text(view.TenantIcon(tenant.Name))),
						g.Text(" "),
						g.Text(tenant.Name+" / "+environment.Name),
					),
					h.P(h.Class("text-muted"), g.Textf("%d features enabled", countEnabled(enabledFeatures))),
					h.H2(g.Text("Environment Metadata")),
					h.Table(h.Class("table"),
						h.TBody(g.Group(g.Map(environment.Metadata, func(item MetadataItem) g.Node {
							return h.Tr(
								h.Td(h.Class("th-like width-md"), g.Text(item.Key)),
								h.Td(g.Text(item.Value)),
							)
						}))),
					),
				),
			),
		),
	)
}
