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
	"github.com/nais/fasit/internal/rollout"
	"github.com/nais/fasit/internal/ui"
	uiserver "github.com/nais/fasit/internal/ui/server"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

func SetupRouter(
	ctx context.Context,
	loadContext contextloader.LoaderFunc,
	iapAudience string,
	insecureSkipProxy, insecureSkipTokenCheck bool,
	graphHandler http.Handler,
	repo database.Repo,
	log logrus.FieldLogger,
) (http.Handler, error) {
	// Add the IAP validation middleware.
	// If the IAP audience is not set, we stop the server with a fatal error
	// unless the INSECURE_SKIP_PROXY env var is true.
	iapMW := auth.ValidateJWTFromComputeEngine(iapAudience)
	if iapAudience == "" {
		if !insecureSkipProxy {
			return nil, fmt.Errorf("INSECURE_SKIP_PROXY must be true when iap audience is not set")
		}
		iapMW = auth.InsecureValidateMW
	}

	router := chi.NewMux()
	router.Use(contextMiddleware(loadContext))
	router.Handle("/query", iapMW(graphHandler))
	router.Handle("/metrics", promhttp.Handler())

	rout, err := rollout.New(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("error creating rollout handler: %w", err)
	}
	rout.AllowAll = insecureSkipTokenCheck
	router.Post("/github/rollout", rout.Rollout)

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
		// return a contextMiddleware that injects the loader to the request context
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Note that the loaders are being created per-request. This is important because they contain caching and
			// batching logic that must be request-scoped.
			r = r.WithContext(fn(r.Context()))
			next.ServeHTTP(w, r)
		})
	}
}
