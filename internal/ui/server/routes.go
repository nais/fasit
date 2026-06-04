package server

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/nais/fasit/internal/ui/pages/assignments"
	"github.com/nais/fasit/internal/ui/pages/auditlog"
	"github.com/nais/fasit/internal/ui/pages/environment"
	"github.com/nais/fasit/internal/ui/pages/features"
	"github.com/nais/fasit/internal/ui/pages/labels"
	reconcilerpage "github.com/nais/fasit/internal/ui/pages/reconciler"
	"github.com/nais/fasit/internal/ui/pages/templatetest"
	"github.com/nais/fasit/internal/ui/pages/tenants"
)

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	mm, err := NewMetricsMiddleware(s.meter)
	if err != nil {
		slog.With("err", err).Warn("failed to create HTTP metrics middleware")
	} else {
		r.Use(mm.Handler)
	}

	r.Get("/site.css", s.CSS)
	r.Get("/site.js", s.JS)
	r.Get("/assignments.js", s.PageJS)
	r.Get("/features.js", s.PageJS)
	r.Get("/reconciler.js", s.PageJS)
	r.Get("/favicon.ico", s.Favicon)

	r.Get("/", features.ListHandler(s.renderPage))
	r.Get("/environments", tenants.Handler(s.renderPage))
	r.Get("/tenants/{tenant}/logo", tenants.ServeLogoHandler())

	r.Get("/tenants/{tenant}/envs/{env}", environment.Handler(s.renderPage))

	r.Get("/assignments", assignments.ListHandler(s.renderPage))
	r.Post("/assignments", assignments.CreateHandler())
	r.Post("/assignments/preview-targets", assignments.PreviewTargetsHandler())
	r.Get("/assignments/{id}", assignments.DetailHandler(s.renderPage))
	r.Get("/assignments/{id}/logs/{envID}", assignments.LogsHandler(s.renderPage))
	r.Post("/assignments/{id}/deactivate", assignments.DeactivateHandler())
	r.Post("/assignments/{id}/deactivate-matching", assignments.DeactivateByFeatureAndTargetHandler())

	r.Get("/features", features.IndexHandler(s.renderPage))
	r.Get("/features/{feature}", features.Handler(s.renderPage))
	r.Get("/features/{feature}/assignments", features.DeploySpecsHandler(s.renderPage))
	r.Get("/features/{feature}/config", features.ConfigTabHandler(s.renderPage))
	r.Get("/features/{feature}/config-compare/{key}", features.ConfigCompareHandler())
	r.Get("/features/{feature}/envs/{tenant}/{env}", environment.FeatureContextTabHandler(s.renderPage, "status"))
	r.Get("/features/{feature}/envs/{tenant}/{env}/config", environment.FeatureContextTabHandler(s.renderPage, "config"))
	r.Get("/features/{feature}/envs/{tenant}/{env}/logs", environment.FeatureLogsRedirectHandler())
	r.Get("/features/{feature}/envs/{tenant}/{env}/helm-values", environment.HelmValuesFragmentHandler())
	r.Get("/features/{feature}/envs/{tenant}/{env}/helm", environment.LegacyFeatureRedirectHandler("/config"))
	r.Get("/features/{feature}/envs/{tenant}/{env}/assignments", environment.FeatureContextTabHandler(s.renderPage, "assignments"))
	r.Get("/features/{feature}/envs/{tenant}/{env}/audit", environment.AuditRedirectHandler())
	r.Get("/features/{feature}/envs/{tenant}/{env}/playground", environment.LegacyFeatureRedirectHandler("/config"))
	r.Post("/features/{feature}/envs/{tenant}/{env}/playground", environment.LegacyFeatureRedirectHandler("/config"))
	r.Get("/features/{feature}/envs/{tenant}/{env}/config/edit/{id}", environment.FeatureContextTabHandler(s.renderPage, "config"))
	r.Post("/features/{feature}/envs/{tenant}/{env}/config/edit/{id}", environment.UpdateConfigHandler())
	r.Post("/features/{feature}/envs/{tenant}/{env}/config/delete/{id}", environment.DeleteConfigHandler())
	r.Get("/features/{feature}/envs/{tenant}/{env}/config/override", environment.FeatureContextTabHandler(s.renderPage, "config"))
	r.Post("/features/{feature}/envs/{tenant}/{env}/config/override", environment.ConfigOverrideSubmitHandler())
	r.Post("/features/{feature}/envs/{tenant}/{env}/toggle-reconcile", environment.ToggleFeatureStateHandler())
	r.Post("/features/{feature}/envs/{tenant}/{env}/redeploy", environment.RedeployHandler())
	r.Post("/features/{feature}/config/{id}", features.UpdateGlobalConfigHandler())
	r.Post("/features/{feature}/config/{id}/delete", features.DeleteGlobalConfigHandler())
	r.Post("/features/{feature}/config/set", features.SetGlobalConfigHandler())

	r.Get("/auditlog", auditlog.Handler(s.renderPage))

	r.Get("/template-test", templatetest.Handler(s.renderPage))

	r.Get("/labels", labels.Handler(s.renderPage))

	r.Get("/reconciler", reconcilerpage.Handler(s.renderPage))
	r.Get("/reconciler/stream", reconcilerpage.StreamHandler())

	return r
}
