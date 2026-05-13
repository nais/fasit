package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/nais/fasit/internal/ui/pages/deployments"
	"github.com/nais/fasit/internal/ui/pages/environment"
	"github.com/nais/fasit/internal/ui/pages/features"
	"github.com/nais/fasit/internal/ui/pages/labels"
	"github.com/nais/fasit/internal/ui/pages/naisd"
	"github.com/nais/fasit/internal/ui/pages/tenant"
	"github.com/nais/fasit/internal/ui/pages/tenants"
)

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(MetricsMiddleware)

	r.Get("/site.css", s.CSS)
	r.Get("/site.js", s.JS)
	r.Get("/favicon.ico", s.Favicon)

	r.Get("/", tenants.Handler(s.renderPage, s.repo))
	r.Get("/tenants/{tenant}", tenant.Handler(s.renderPage, s.repo))

	r.Get("/tenants/{tenant}/envs/{env}", environment.Handler(s.renderPage, s.repo))

	r.Get("/tenants/{tenant}/envs/{env}/{feature}", environment.FeatureTabHandler(s.renderPage, s.repo, "overview"))
	r.Get("/tenants/{tenant}/envs/{env}/{feature}/logs", environment.FeatureTabHandler(s.renderPage, s.repo, "logs"))
	r.Get("/tenants/{tenant}/envs/{env}/{feature}/helm", environment.FeatureTabHandler(s.renderPage, s.repo, "helm"))
	r.Get("/tenants/{tenant}/envs/{env}/{feature}/deployments", environment.FeatureTabHandler(s.renderPage, s.repo, "deployments"))
	r.Get("/tenants/{tenant}/envs/{env}/{feature}/audit", environment.FeatureTabHandler(s.renderPage, s.repo, "audit"))
	r.Get("/tenants/{tenant}/envs/{env}/{feature}/playground", environment.PlaygroundTabHandler(s.renderPage, s.repo))
	r.Post("/tenants/{tenant}/envs/{env}/{feature}/playground", environment.PlaygroundSubmitHandler(s.renderPage, s.repo))

	r.Get("/tenants/{tenant}/envs/{env}/{feature}/config/edit/{id}", environment.FeatureTabHandler(s.renderPage, s.repo, "overview"))
	r.Post("/tenants/{tenant}/envs/{env}/{feature}/config/edit/{id}", environment.UpdateConfigHandler(s.repo))
	r.Post("/tenants/{tenant}/envs/{env}/{feature}/config/delete/{id}", environment.DeleteConfigHandler(s.repo))
	r.Get("/tenants/{tenant}/envs/{env}/{feature}/config/override", environment.FeatureTabHandler(s.renderPage, s.repo, "overview"))
	r.Post("/tenants/{tenant}/envs/{env}/{feature}/config/override", environment.ConfigOverrideSubmitHandler(s.repo))
	r.Post("/tenants/{tenant}/envs/{env}/{feature}/toggle-reconcile", environment.ToggleFeatureStateHandler(s.repo))
	r.Post("/tenants/{tenant}/envs/{env}/{feature}/redeploy", environment.RedeployHandler(s.repo))

	r.Get("/deployments", deployments.ListHandler(s.renderPage))
	r.Post("/deployments", deployments.CreateHandler())
	r.Get("/deployments/{id}", deployments.DetailHandler(s.renderPage, s.repo))
	r.Get("/deployments/{id}/logs/{envID}", deployments.LogsHandler(s.renderPage))
	r.Post("/deployments/{id}/delete", deployments.DeleteHandler())
	r.Post("/deployments/{id}/delete-matching", deployments.DeleteByFeatureAndTargetHandler())
	r.Post("/reconcile", deployments.ReconcileHandler())

	r.Get("/features", features.ListHandler(s.renderPage, s.repo))
	r.Get("/features/{feature}", features.Handler(s.renderPage, s.repo))

	r.Get("/labels", labels.Handler(s.renderPage, s.repo))
	r.Get("/naisd", naisd.Handler(s.renderPage, s.repo))

	return r
}
