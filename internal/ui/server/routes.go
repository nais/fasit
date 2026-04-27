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

	mux.HandleFunc("GET /site.css", s.CSS)
	mux.HandleFunc("GET /site.js", s.JS)

	mux.HandleFunc("GET /rollouts/", rollouts.Handler(s.renderPage, s.repo))
	mux.HandleFunc("GET /rollouts/{feature}/{version}/", rollouts.DetailHandler(s.renderPage, s.repo))
	mux.HandleFunc("GET /deployments/{id}/", rollouts.DeploymentHandler(s.renderPage, s.repo))

	mux.HandleFunc("GET /", tenants.Handler(s.renderPage, s.repo))
	mux.HandleFunc("GET /tenants/{tenant}/", tenant.Handler(s.renderPage, s.repo))

	mux.HandleFunc("GET /tenants/{tenant}/envs/{env}/", environment.Handler(s.renderPage, s.repo))

	mux.HandleFunc("GET /tenants/{tenant}/envs/{env}/{feature}/", environment.FeatureTabHandler(s.renderPage, s.repo, "overview"))
	mux.HandleFunc("GET /tenants/{tenant}/envs/{env}/{feature}/logs", environment.FeatureTabHandler(s.renderPage, s.repo, "logs"))
	mux.HandleFunc("GET /tenants/{tenant}/envs/{env}/{feature}/helm", environment.FeatureTabHandler(s.renderPage, s.repo, "helm"))
	mux.HandleFunc("GET /tenants/{tenant}/envs/{env}/{feature}/rollouts", environment.FeatureTabHandler(s.renderPage, s.repo, "rollouts"))
	mux.HandleFunc("GET /tenants/{tenant}/envs/{env}/{feature}/audit", environment.FeatureTabHandler(s.renderPage, s.repo, "audit"))

	mux.HandleFunc("GET /tenants/{tenant}/envs/{env}/{feature}/config/edit/{id}", environment.FeatureTabHandler(s.renderPage, s.repo, "overview"))
	mux.HandleFunc("POST /tenants/{tenant}/envs/{env}/{feature}/config/edit/{id}", environment.UpdateConfigHandler(s.repo))
	mux.HandleFunc("POST /tenants/{tenant}/envs/{env}/{feature}/config/delete/{id}", environment.DeleteConfigHandler(s.repo))
	mux.HandleFunc("GET /tenants/{tenant}/envs/{env}/{feature}/config/override", environment.FeatureTabHandler(s.renderPage, s.repo, "overview"))
	mux.HandleFunc("POST /tenants/{tenant}/envs/{env}/{feature}/config/override", environment.ConfigOverrideSubmitHandler(s.repo))
	mux.HandleFunc("POST /tenants/{tenant}/envs/{env}/{feature}/toggle-reconcile", environment.ToggleFeatureStateHandler(s.repo))

	mux.HandleFunc("GET /features/", features.ListHandler(s.renderPage, s.repo))
	mux.HandleFunc("GET /features/{feature}/", features.TabHandler(s.renderPage, s.repo, "overview"))
	mux.HandleFunc("GET /features/{feature}/status", features.TabHandler(s.renderPage, s.repo, "status"))
	mux.HandleFunc("GET /features/{feature}/rollouts", features.TabHandler(s.renderPage, s.repo, "rollouts"))

	mux.HandleFunc("GET /tenants/{tenant}/envs/{env}/{feature}/playground", environment.PlaygroundTabHandler(s.renderPage, s.repo))
	mux.HandleFunc("POST /tenants/{tenant}/envs/{env}/{feature}/playground", environment.PlaygroundSubmitHandler(s.renderPage, s.repo))

	return LoggingMiddleware(mux)
}
