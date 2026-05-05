package tenants

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/nais/fasit/internal/ui/view"
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
	Tenant       *model.Tenant
	Environments []envCard
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

			envCards := make([]envCard, 0, len(envs))
			for _, env := range envs {
				failed, pending := environmentStatusCounts(r.Context(), repo, env)
				envCards = append(envCards, envCard{
					Environment: env,
					Failed:      failed,
					Pending:     pending,
				})
			}

			cards = append(cards, tenantCard{
				Tenant:       tenant,
				Environments: envCards,
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

// environmentStatusCounts returns the number of features in this environment
// whose latest deploy instruction is failed or pending. Deploy instructions
// unify rollout-driven and deployment-driven progress, so this single lookup
// covers both paths.
func environmentStatusCounts(ctx context.Context, repo database.Repo, env *model.Environment) (failed, pending int) {
	deploymentFeatures, err := deployment.ListEnvironmentFeatures(ctx, env.ID)
	if err != nil {
		return 0, 0
	}
	seen := make(map[string]bool, len(deploymentFeatures))
	for _, f := range deploymentFeatures {
		seen[f.FeatureName] = true
	}
	states, err := featurepkg.FeatureStatesGet(ctx, env.ID)
	if err != nil {
		return 0, 0
	}
	for _, state := range states {
		if !seen[state.FeatureName] {
			seen[state.FeatureName] = true
		}
	}
	for name := range seen {
		f, p := featureStatusForEnv(ctx, repo, env.ID, name)
		if f {
			failed++
		} else if p {
			pending++
		}
	}
	return failed, pending
}

func featureStatusForEnv(ctx context.Context, repo database.Repo, envID uuid.UUID, featureName string) (failed, pending bool) {
	di, err := repo.DeployInstructionsLatestForFeature(ctx, envID, featureName)
	if err != nil || di == nil {
		return false, false
	}
	switch di.Status {
	case model.RolloutStatusFailed:
		return true, false
	case model.RolloutStatusPending, model.RolloutStatusCreated:
		return false, true
	}
	return false, false
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

func tenantBadge(tenant tenantCard) g.Node {
	return h.Span(
		h.Class("tenant-icon"),
		h.Style("background:"+tenant.IconColor),
		g.Text(tenant.Icon),
	)
}
