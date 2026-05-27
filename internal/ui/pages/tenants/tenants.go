package tenants

import (
	"net/http"

	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/database"
	"github.com/nais/fasit/internal/ui/layout"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, *http.Request, layout.Props)

type envCard struct {
	Environment *model.Environment
	Failed      int
	Pending     int
}

type tenantCard struct {
	Tenant       *database.Tenant
	Environments []envCard
}

func Handler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenants, err := database.ListTenants(r.Context())
		if err != nil {
			http.Error(w, "Failed to load tenants", http.StatusInternalServerError)
			return
		}

		cards := make([]tenantCard, 0, len(tenants))
		for _, tenant := range tenants {
			envs, err := environment.List(r.Context(), tenant.ID)
			if err != nil {
				http.Error(w, "Failed to load tenants", http.StatusInternalServerError)
				return
			}

			envCards := make([]envCard, 0, len(envs))
			for _, env := range envs {
				envCards = append(envCards, envCard{
					Environment: env,
				})
			}

			cards = append(cards, tenantCard{
				Tenant:       tenant,
				Environments: envCards,
			})
		}

		renderPage(w, r, layout.Props{
			Title:       "Environments",
			CurrentPage: components.PageEnvironments,
			Content:     page(cards),
		})
	}
}

func page(tenants []tenantCard) g.Node {
	articles := g.Map(tenants, func(tenant tenantCard) g.Node {
		hasLogo := components.HasTenantLogo(tenant.Tenant.Name)
		return h.Article(h.Class("dash-card"),
			h.H3(
				components.TenantAvatar(tenant.Tenant.Name, hasLogo, "36px"),
				g.Text(tenant.Tenant.Name),
			),
			h.Ul(g.Group(g.Map(tenant.Environments, func(envc envCard) g.Node {
				return h.Li(
					h.A(h.Href("/tenants/"+tenant.Tenant.Name+"/envs/"+envc.Environment.Name),
						g.Text(envc.Environment.Name),
						components.StatusCountsBadge(envc.Failed, envc.Pending),
					),
				)
			}))),
		)
	})

	return h.Div(h.Class("dashboard"), g.Group(articles))
}
