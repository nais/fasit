package tenants

import (
	"context"
	"net/http"

	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/nais/fasit/internal/ui/uidata"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, *http.Request, layout.Props)

func Handler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenants, err := uidata.ListTenants(r.Context())
		if err != nil {
			http.Error(w, "Failed to load tenants", http.StatusInternalServerError)
			return
		}

		renderPage(w, r, layout.Props{
			Title:       "Environments",
			CurrentPage: components.PageEnvironments,
			Content:     page(r.Context(), tenants),
		})
	}
}

func page(ctx context.Context, tenants []*uidata.Tenant) g.Node {
	articles := g.Map(tenants, func(tenant *uidata.Tenant) g.Node {
		hasLogo := components.HasTenantLogo(tenant.Name)
		return h.Article(h.Class("dash-card"),
			h.H3(
				components.TenantAvatar(tenant.Name, hasLogo, "36px"),
				g.Text(tenant.Name),
			),
			h.Ul(components.Map(ctx, tenant.Environments, func(env *uidata.Environment) g.Node {
				return h.Li(
					h.A(h.Href("/tenants/"+tenant.Name+"/envs/"+env.Name),
						g.Text(env.Name),
					),
				)
			})),
		)
	})

	return h.Div(h.Class("dashboard"), articles)
}
