package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/nais/fasit/internal/ui/pages/deployments"
	"github.com/nais/fasit/internal/ui/pages/environment"
	"github.com/nais/fasit/internal/ui/pages/features"
	"github.com/nais/fasit/internal/ui/pages/labels"
	"github.com/nais/fasit/internal/ui/pages/naisd"
	"github.com/nais/fasit/internal/ui/pages/tenants"
	"github.com/sirupsen/logrus"
)

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	mm, err := NewMetricsMiddleware(s.meter)
	if err != nil {
		logrus.WithError(err).Warn("failed to create HTTP metrics middleware")
	} else {
		r.Use(mm.Handler)
	}

	r.Get("/site.css", s.CSS)
	r.Get("/site.js", s.JS)
	r.Get("/favicon.ico", s.Favicon)

	r.Get("/", features.ListHandler(s.renderPage))
	r.Get("/environments", tenants.Handler(s.renderPage))

	r.Get("/tenants/{tenant}/envs/{env}", environment.Handler(s.renderPage))

	r.Get("/tenants/{tenant}/envs/{env}/{feature}", environment.FeatureTabHandler(s.renderPage, "overview"))
	r.Get("/tenants/{tenant}/envs/{env}/{feature}/logs", environment.FeatureTabHandler(s.renderPage, "logs"))
	r.Get("/tenants/{tenant}/envs/{env}/{feature}/helm", environment.FeatureTabHandler(s.renderPage, "helm"))
	r.Get("/tenants/{tenant}/envs/{env}/{feature}/deployments", environment.FeatureTabHandler(s.renderPage, "deployments"))
	r.Get("/tenants/{tenant}/envs/{env}/{feature}/audit", environment.FeatureTabHandler(s.renderPage, "audit"))
	r.Get("/tenants/{tenant}/envs/{env}/{feature}/playground", environment.PlaygroundTabHandler(s.renderPage))
	r.Post("/tenants/{tenant}/envs/{env}/{feature}/playground", environment.PlaygroundSubmitHandler(s.renderPage))

	r.Get("/tenants/{tenant}/envs/{env}/{feature}/config/edit/{id}", environment.FeatureTabHandler(s.renderPage, "overview"))
	r.Post("/tenants/{tenant}/envs/{env}/{feature}/config/edit/{id}", environment.UpdateConfigHandler())
	r.Post("/tenants/{tenant}/envs/{env}/{feature}/config/delete/{id}", environment.DeleteConfigHandler())
	r.Get("/tenants/{tenant}/envs/{env}/{feature}/config/override", environment.FeatureTabHandler(s.renderPage, "overview"))
	r.Post("/tenants/{tenant}/envs/{env}/{feature}/config/override", environment.ConfigOverrideSubmitHandler())
	r.Post("/tenants/{tenant}/envs/{env}/{feature}/toggle-reconcile", environment.ToggleFeatureStateHandler())
	r.Post("/tenants/{tenant}/envs/{env}/{feature}/redeploy", environment.RedeployHandler())

	r.Get("/deployments", deployments.ListHandler(s.renderPage))
	r.Post("/deployments", deployments.CreateHandler())
	r.Post("/deployments/preview-targets", deployments.PreviewTargetsHandler())
	r.Get("/deployments/{id}", deployments.DetailHandler(s.renderPage))
	r.Get("/deployments/{id}/logs/{envID}", deployments.LogsHandler(s.renderPage))
	r.Post("/deployments/{id}/delete", deployments.DeleteHandler())
	r.Post("/deployments/{id}/delete-matching", deployments.DeleteByFeatureAndTargetHandler())
	r.Post("/reconcile", deployments.ReconcileHandler())

	r.Get("/features", features.ListHandler(s.renderPage))
	r.Get("/features/{feature}", features.Handler(s.renderPage))
	r.Get("/features/{feature}/config", features.ConfigTabHandler(s.renderPage))
	r.Post("/features/{feature}/config/{id}", features.UpdateGlobalConfigHandler())
	r.Post("/features/{feature}/config/{id}/delete", features.DeleteGlobalConfigHandler())
	r.Post("/features/{feature}/config/set", features.SetGlobalConfigHandler())

	r.Get("/labels", labels.Handler(s.renderPage))
	r.Get("/naisd", naisd.Handler(s.renderPage))

	return r
}
