package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/nais/fasit/internal/ui/pages/auditlog"
	"github.com/nais/fasit/internal/ui/pages/deployments"
	"github.com/nais/fasit/internal/ui/pages/environment"
	"github.com/nais/fasit/internal/ui/pages/features"
	"github.com/nais/fasit/internal/ui/pages/labels"
	"github.com/nais/fasit/internal/ui/pages/naisd"
	reconcilerpage "github.com/nais/fasit/internal/ui/pages/reconciler"
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
	r.Get("/deployments.js", s.PageJS)
	r.Get("/features.js", s.PageJS)
	r.Get("/reconciler.js", s.PageJS)
	r.Get("/favicon.ico", s.Favicon)

	r.Get("/", features.ListHandler(s.renderPage))
	r.Get("/environments", tenants.Handler(s.renderPage))
	r.Get("/tenants/{tenant}/logo", tenants.ServeLogoHandler())

	r.Get("/tenants/{tenant}/envs/{env}", environment.Handler(s.renderPage))

	r.Get("/tenants/{tenant}/envs/{env}/{feature}", environment.LegacyFeatureRedirectHandler(""))
	r.Get("/tenants/{tenant}/envs/{env}/{feature}/logs", environment.LegacyFeatureRedirectHandler("/logs"))
	r.Get("/tenants/{tenant}/envs/{env}/{feature}/helm", environment.LegacyFeatureRedirectHandler("/helm"))
	r.Get("/tenants/{tenant}/envs/{env}/{feature}/deployments", environment.LegacyFeatureRedirectHandler("/deployments"))
	r.Get("/tenants/{tenant}/envs/{env}/{feature}/audit", environment.AuditRedirectHandler())
	r.Get("/tenants/{tenant}/envs/{env}/{feature}/playground", environment.LegacyFeatureRedirectHandler("/playground"))
	r.Post("/tenants/{tenant}/envs/{env}/{feature}/playground", environment.LegacyFeatureRedirectHandler("/playground"))

	r.Get("/tenants/{tenant}/envs/{env}/{feature}/config/edit/{id}", environment.LegacyFeatureRedirectHandler("/config/edit/{id}"))
	r.Post("/tenants/{tenant}/envs/{env}/{feature}/config/edit/{id}", environment.UpdateConfigHandler())
	r.Post("/tenants/{tenant}/envs/{env}/{feature}/config/delete/{id}", environment.DeleteConfigHandler())
	r.Get("/tenants/{tenant}/envs/{env}/{feature}/config/override", environment.LegacyFeatureRedirectHandler("/config/override"))
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

	r.Get("/features", features.IndexHandler(s.renderPage))
	r.Get("/features/{feature}", features.Handler(s.renderPage))
	r.Get("/features/{feature}/deploy-specs", features.DeploySpecsHandler(s.renderPage))
	r.Get("/features/{feature}/config", features.ConfigTabHandler(s.renderPage))
	r.Get("/features/{feature}/config-explorer", features.ConfigExplorerHandler(s.renderPage))
	r.Get("/features/{feature}/envs/{tenant}/{env}", environment.FeatureContextTabHandler(s.renderPage, "overview"))
	r.Get("/features/{feature}/envs/{tenant}/{env}/logs", environment.FeatureContextTabHandler(s.renderPage, "logs"))
	r.Get("/features/{feature}/envs/{tenant}/{env}/helm", environment.FeatureContextTabHandler(s.renderPage, "helm"))
	r.Get("/features/{feature}/envs/{tenant}/{env}/deployments", environment.FeatureContextTabHandler(s.renderPage, "deployments"))
	r.Get("/features/{feature}/envs/{tenant}/{env}/audit", environment.AuditRedirectHandler())
	r.Get("/features/{feature}/envs/{tenant}/{env}/playground", environment.FeatureContextPlaygroundTabHandler(s.renderPage))
	r.Post("/features/{feature}/envs/{tenant}/{env}/playground", environment.FeatureContextPlaygroundSubmitHandler(s.renderPage))
	r.Get("/features/{feature}/envs/{tenant}/{env}/config/edit/{id}", environment.FeatureContextTabHandler(s.renderPage, "overview"))
	r.Post("/features/{feature}/envs/{tenant}/{env}/config/edit/{id}", environment.UpdateConfigHandler())
	r.Post("/features/{feature}/envs/{tenant}/{env}/config/delete/{id}", environment.DeleteConfigHandler())
	r.Get("/features/{feature}/envs/{tenant}/{env}/config/override", environment.FeatureContextTabHandler(s.renderPage, "overview"))
	r.Post("/features/{feature}/envs/{tenant}/{env}/config/override", environment.ConfigOverrideSubmitHandler())
	r.Post("/features/{feature}/envs/{tenant}/{env}/toggle-reconcile", environment.ToggleFeatureStateHandler())
	r.Post("/features/{feature}/envs/{tenant}/{env}/redeploy", environment.RedeployHandler())
	r.Post("/features/{feature}/config/{id}", features.UpdateGlobalConfigHandler())
	r.Post("/features/{feature}/config/{id}/delete", features.DeleteGlobalConfigHandler())
	r.Post("/features/{feature}/config/set", features.SetGlobalConfigHandler())

	r.Get("/auditlog", auditlog.Handler(s.renderPage))

	r.Get("/labels", labels.Handler(s.renderPage))
	r.Get("/naisd", naisd.Handler(s.renderPage))

	r.Get("/reconciler", reconcilerpage.Handler(s.renderPage))
	r.Get("/reconciler/stream", reconcilerpage.StreamHandler())

	return r
}
