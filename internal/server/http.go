package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/nais/fasit/internal/auth"
	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/ui"
	uiserver "github.com/nais/fasit/internal/ui/server"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

func SetupRouter(
	ctx context.Context,
	loadContext contextloader.LoaderFunc,
	iapAudience string,
	insecureSkipProxy bool,
	repo database.Repo,
	log logrus.FieldLogger,
) (http.Handler, error) {
	iapMW := auth.ValidateJWTFromComputeEngine(iapAudience)
	if iapAudience == "" {
		if !insecureSkipProxy {
			return nil, fmt.Errorf("INSECURE_SKIP_PROXY must be true when iap audience is not set")
		}
		iapMW = auth.InsecureValidateMW
	}

	router := chi.NewMux()
	router.Use(contextMiddleware(loadContext))
	router.Handle("/metrics", promhttp.Handler())

	deploy, err := deployment.NewHttpHandler(ctx, log)
	if err != nil {
		return nil, fmt.Errorf("error creating deployment http handler: %w", err)
	}
	deploy.AllowAll = insecureSkipProxy
	router.Post("/github/deployment", deploy.CreateDeployment)
	router.Get("/github/deployment/{id}", deploy.GetDeployment)
	uiServer := uiserver.New(ui.SiteFS, repo)
	router.Mount("/", iapMW(uiServer.Routes()))
	return router, nil
}

func contextMiddleware(fn func(context.Context) context.Context) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Note that the loaders are being created per-request. This is important because they contain caching and
			// batching logic that must be request-scoped.
			r = r.WithContext(fn(r.Context()))
			next.ServeHTTP(w, r)
		})
	}
}
