package server

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/ui/pages/assignments"
	"github.com/nais/fasit/internal/ui/pages/auditlog"
	"github.com/nais/fasit/internal/ui/pages/environment"
	"github.com/nais/fasit/internal/ui/pages/environments"
	"github.com/nais/fasit/internal/ui/pages/features"
	"github.com/nais/fasit/internal/ui/pages/labels"
	reconcilerpage "github.com/nais/fasit/internal/ui/pages/reconciler"
	"github.com/nais/fasit/internal/ui/pages/templatetest"
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
	r.Get("/reconciler.js", s.PageJS)
	r.Get("/favicon.ico", s.Favicon)
	r.Get("/favicon.svg", s.FaviconSVG)

	r.Get("/", features.ListHandler(s.renderPage))
	r.Get("/environments", environments.Handler(s.renderPage))
	r.Get("/tenants/{tenant}/logo", environments.ServeLogoHandler())

	r.Get("/tenants/{tenant}/envs/{env}", environment.Handler(s.renderPage))
	r.Post("/tenants/{tenant}/envs/{env}/releases/{release}/uninstall", environment.UninstallReleaseHandler())

	r.Get("/assignments", assignments.ListHandler(s.renderPage))
	r.Post("/assignments", assignments.CreateHandler())
	r.Post("/assignments/preview-targets", assignments.PreviewTargetsHandler())
	r.Get("/assignments/{id}/logs/{envID}", assignments.LogsHandler(s.renderPage))
	r.Post("/assignments/{id}/deactivate", assignments.DeactivateHandler())
	r.Post("/assignments/{id}/deactivate-matching", assignments.DeactivateByFeatureAndTargetHandler())

	// Legacy redirects, needed until fasit-deploy is updated.
	r.Get("/deployments", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/assignments", http.StatusMovedPermanently)
	})
	r.Get("/deployments/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			http.Redirect(w, r, "/assignments", http.StatusSeeOther)
			return
		}
		d, err := featureassignment.Get(r.Context(), id)
		if err != nil {
			http.Redirect(w, r, "/assignments", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/features/"+d.Feature.Name+"/assignments/"+id.String(), http.StatusMovedPermanently)
	})

	r.Get("/features", features.IndexHandler(s.renderPage))
	r.Get("/features/{feature}", features.Handler(s.renderPage))
	r.Get("/features/{feature}/assignments", features.DeploySpecsHandler(s.renderPage))
	r.Get("/features/{feature}/assignments/{id}", features.AssignmentDetailHandler(s.renderPage))
	r.Get("/features/{feature}/versions", features.VersionsTabHandler(s.renderPage))
	r.Get("/features/{feature}/versions/{version}", features.VersionDetailHandler(s.renderPage))
	r.Get("/features/{feature}/config", features.ConfigTabHandler(s.renderPage))
	r.Get("/features/{feature}/config-compare/{key}", features.ConfigCompareHandler())
	r.Get("/features/{feature}/envs/{tenant}/{env}", environment.FeatureContextTabHandler(s.renderPage, "status"))
	r.Get("/features/{feature}/envs/{tenant}/{env}/config", environment.FeatureContextTabHandler(s.renderPage, "config"))
	r.Get("/features/{feature}/envs/{tenant}/{env}/logs", environment.FeatureLogsRedirectHandler())
	r.Get("/features/{feature}/envs/{tenant}/{env}/helm-values", environment.HelmValuesFragmentHandler())
	r.Get("/features/{feature}/envs/{tenant}/{env}/assignments", environment.FeatureContextTabHandler(s.renderPage, "assignments"))
	r.Get("/features/{feature}/envs/{tenant}/{env}/audit", environment.AuditRedirectHandler())
	r.Get("/features/{feature}/envs/{tenant}/{env}/config/edit/{id}", environment.FeatureContextTabHandler(s.renderPage, "config"))
	r.Post("/features/{feature}/envs/{tenant}/{env}/config/edit/{id}", environment.UpdateConfigHandler())
	r.Post("/features/{feature}/envs/{tenant}/{env}/config/delete/{id}", environment.DeleteConfigHandler())
	r.Get("/features/{feature}/envs/{tenant}/{env}/config/override", environment.FeatureContextTabHandler(s.renderPage, "config"))
	r.Post("/features/{feature}/envs/{tenant}/{env}/config/override", environment.ConfigOverrideSubmitHandler())
	r.Post("/features/{feature}/envs/{tenant}/{env}/config/batch", environment.BatchUpdateConfigHandler())
	r.Post("/features/{feature}/envs/{tenant}/{env}/toggle-reconcile", environment.ToggleFeatureStateHandler())
	r.Post("/features/{feature}/envs/{tenant}/{env}/redeploy", environment.RedeployHandler())
	r.Post("/features/{feature}/config/{id}", features.UpdateGlobalConfigHandler())
	r.Post("/features/{feature}/config/{id}/delete", features.DeleteGlobalConfigHandler())
	r.Post("/features/{feature}/config/set", features.SetGlobalConfigHandler())

	r.Get("/auditlog", auditlog.Handler(s.renderPage))
	r.Get("/auditlog/uninstall-logs/{diid}", auditlog.UninstallLogsFragmentHandler())

	r.Get("/template-test", templatetest.Handler(s.renderPage))

	r.Get("/labels", labels.Handler(s.renderPage))

	r.Get("/reconciler", reconcilerpage.Handler(s.renderPage))
	r.Get("/reconciler/stream", reconcilerpage.StreamHandler())

	return r
}
