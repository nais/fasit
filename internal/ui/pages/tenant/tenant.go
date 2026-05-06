package tenant

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/ui/breadcrumb"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, *http.Request, layout.Props)

type envRow struct {
	Environment *model.Environment
	Failed      int
	Pending     int
}

type pageData struct {
	Tenant       *model.Tenant
	Environments []envRow
	Icon         string
	IconColor    string
}

func Handler(renderPage RenderPage, repo database.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "tenant")

		tenant, err := environment.GetTenantGetByName(r.Context(), slug)
		if err != nil {
			http.Error(w, "Failed to load tenant: "+err.Error(), http.StatusInternalServerError)
			return
		}

		envs, err := repo.EnvironmentsGet(r.Context(), tenant.ID)
		if err != nil {
			http.Error(w, "Failed to load tenant: "+err.Error(), http.StatusInternalServerError)
			return
		}

		envRows := make([]envRow, 0, len(envs))
		for _, env := range envs {
			failed, pending := 0, 0
			envRows = append(envRows, envRow{
				Environment: env,
				Failed:      failed,
				Pending:     pending,
			})
		}

		allTenants, _ := environment.GetTenants(r.Context())
		breadcrumbs := []breadcrumb.Crumb{
			breadcrumb.TenantWithSwitcher(tenant.Name, toTenantNavs(allTenants)),
		}

		data := pageData{
			Tenant:       tenant,
			Environments: envRows,
			Icon:         view.TenantIcon(tenant.Name),
			IconColor:    view.TenantColor(tenant.Name),
		}

		renderPage(w, r, layout.Props{
			Title:       tenant.Name,
			CurrentPage: components.PageTenants,
			Content:     page(breadcrumbs, data),
		})
	}
}

func page(breadcrumbs []breadcrumb.Crumb, tenant pageData) g.Node {
	return h.Div(h.Class("container"),
		h.Main(h.Class("main-content"),
			components.Breadcrumbs(breadcrumbs),
			h.Div(h.Class("card"),
				h.Div(h.Class("card-body"),
					h.H1(
						h.Span(
							h.Class("tenant-icon tenant-icon-lg"),
							h.Style("background:"+tenant.IconColor),
							g.Text(tenant.Icon),
						),
						g.Text(" "),
						g.Text(tenant.Tenant.Name),
					),
					h.H2(g.Text("Environments")),
					h.Table(h.Class("table sortable"),
						h.THead(
							h.Tr(
								h.Th(g.Text("Name")),
								h.Th(g.Text("Description")),
								h.Th(g.Text("Kind")),
								h.Th(g.Text("Reconcile")),
							),
						),
						h.TBody(g.Group(g.Map(tenant.Environments, func(row envRow) g.Node {
							return h.Tr(
								h.Td(h.A(h.Href("/tenants/"+tenant.Tenant.Name+"/envs/"+row.Environment.Name),
									g.Text(row.Environment.Name),
									components.StatusCountsBadge(row.Failed, row.Pending),
								)),
								h.Td(g.Text(valueOrEmpty(row.Environment.Description))),
								h.Td(g.Text(row.Environment.Kind.String())),
								h.Td(g.Text(checkmarkOrDash(row.Environment.Reconcile))),
							)
						}))),
					),
				),
			),
		),
	)
}

func toTenantNavs(tenants []*model.Tenant) []view.TenantNav {
	ret := make([]view.TenantNav, 0, len(tenants))
	for _, tenant := range tenants {
		ret = append(ret, view.TenantNav{Name: tenant.Name})
	}
	return ret
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func checkmarkOrDash(ok bool) string {
	if ok {
		return "✓"
	}
	return "-"
}
