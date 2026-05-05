package tenants

import (
	"net/http"

	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, *http.Request, layout.Props)

type tenantCard struct {
	Tenant       *model.Tenant
	Environments []*model.Environment
	Icon         string
	IconColor    string
}

func Handler(renderPage RenderPage, repo database.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenants, err := environment.GetTenants(r.Context())
		if err != nil {
			http.Error(w, "Failed to load tenants", http.StatusInternalServerError)
			return
		}

		cards := make([]tenantCard, 0, len(tenants))
		for _, tenant := range tenants {
			envs, err := repo.EnvironmentsGet(r.Context(), tenant.ID)
			if err != nil {
				http.Error(w, "Failed to load tenants", http.StatusInternalServerError)
				return
			}

			cards = append(cards, tenantCard{
				Tenant:       tenant,
				Environments: envs,
				Icon:         view.TenantIcon(tenant.Name),
				IconColor:    view.TenantColor(tenant.Name),
			})
		}

		renderPage(w, r, layout.Props{
			Title:       "Tenants",
			CurrentPage: components.PageTenants,
			Content:     page(cards),
		})
	}
}

func page(tenants []tenantCard) g.Node {
	articles := g.Map(tenants, func(tenant tenantCard) g.Node {
		return h.Article(h.Class("dash-card"),
			h.H3(
				h.A(h.Href("/tenants/"+tenant.Tenant.Name),
					tenantBadge(tenant),
					g.Text(" "),
					g.Text(tenant.Tenant.Name),
				),
			),
			h.Ul(g.Group(g.Map(tenant.Environments, func(environment *model.Environment) g.Node {
				return h.Li(
					h.A(h.Href("/tenants/"+tenant.Tenant.Name+"/envs/"+environment.Name), g.Text(environment.Name)),
				)
			}))),
		)
	})

	return h.Div(h.Class("dashboard"), g.Group(articles))
}

func tenantBadge(tenant tenantCard) g.Node {
	return h.Span(
		h.Class("tenant-icon"),
		h.Style("background:"+tenant.IconColor),
		g.Text(tenant.Icon),
	)
}
