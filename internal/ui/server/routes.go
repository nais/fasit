package server

import (
	"net/http"

	"github.com/nais/fasit/internal/ui/pages/environment"
	"github.com/nais/fasit/internal/ui/pages/features"
	"github.com/nais/fasit/internal/ui/pages/rollouts"
	"github.com/nais/fasit/internal/ui/pages/tenant"
	"github.com/nais/fasit/internal/ui/pages/tenants"
)

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /ui/site.css", s.CSS)
	mux.HandleFunc("GET /ui/site.js", s.JS)

	mux.HandleFunc("GET /ui/rollouts/", rollouts.Handler(s.renderPage, s.repo))
	mux.HandleFunc("GET /ui/rollouts/{feature}/{version}/", rollouts.DetailHandler(s.renderPage, s.repo))
	mux.HandleFunc("GET /ui/deployments/{id}/", rollouts.DeploymentHandler(s.renderPage, s.repo))

	mux.HandleFunc("GET /ui/", tenants.Handler(s.renderPage, s.repo))
	mux.HandleFunc("GET /ui/tenants/{tenant}/", tenant.Handler(s.renderPage, s.repo))

	mux.HandleFunc("GET /ui/tenants/{tenant}/envs/{env}/", environment.Handler(s.renderPage, s.repo))

	mux.HandleFunc("GET /ui/tenants/{tenant}/envs/{env}/{feature}/", environment.FeatureTabHandler(s.renderPage, s.repo, "overview"))
	mux.HandleFunc("GET /ui/tenants/{tenant}/envs/{env}/{feature}/logs", environment.FeatureTabHandler(s.renderPage, s.repo, "logs"))
	mux.HandleFunc("GET /ui/tenants/{tenant}/envs/{env}/{feature}/helm", environment.FeatureTabHandler(s.renderPage, s.repo, "helm"))
	mux.HandleFunc("GET /ui/tenants/{tenant}/envs/{env}/{feature}/rollouts", environment.FeatureTabHandler(s.renderPage, s.repo, "rollouts"))
	mux.HandleFunc("GET /ui/tenants/{tenant}/envs/{env}/{feature}/audit", environment.FeatureTabHandler(s.renderPage, s.repo, "audit"))

	mux.HandleFunc("GET /ui/tenants/{tenant}/envs/{env}/{feature}/config/edit/{id}", environment.FeatureTabHandler(s.renderPage, s.repo, "overview"))
	mux.HandleFunc("POST /ui/tenants/{tenant}/envs/{env}/{feature}/config/edit/{id}", environment.UpdateConfigHandler(s.repo))
	mux.HandleFunc("POST /ui/tenants/{tenant}/envs/{env}/{feature}/config/delete/{id}", environment.DeleteConfigHandler(s.repo))
	mux.HandleFunc("GET /ui/tenants/{tenant}/envs/{env}/{feature}/config/override", environment.FeatureTabHandler(s.renderPage, s.repo, "overview"))
	mux.HandleFunc("POST /ui/tenants/{tenant}/envs/{env}/{feature}/config/override", environment.ConfigOverrideSubmitHandler(s.repo))
	mux.HandleFunc("POST /ui/tenants/{tenant}/envs/{env}/{feature}/toggle-reconcile", environment.ToggleFeatureStateHandler(s.repo))

	mux.HandleFunc("GET /ui/features/", features.ListHandler(s.renderPage, s.repo))
	mux.HandleFunc("GET /ui/features/{feature}/", features.TabHandler(s.renderPage, s.repo, "overview"))
	mux.HandleFunc("GET /ui/features/{feature}/status", features.TabHandler(s.renderPage, s.repo, "status"))
	mux.HandleFunc("GET /ui/features/{feature}/rollouts", features.TabHandler(s.renderPage, s.repo, "rollouts"))

	mux.HandleFunc("GET /ui/tenants/{tenant}/envs/{env}/{feature}/playground", environment.PlaygroundTabHandler(s.renderPage, s.repo))
	mux.HandleFunc("POST /ui/tenants/{tenant}/envs/{env}/{feature}/playground", environment.PlaygroundSubmitHandler(s.renderPage, s.repo))

	return LoggingMiddleware(mux)
}
