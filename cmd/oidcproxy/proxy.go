package main

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

// allowedPaths are the only request paths the proxy forwards. They expose the
// OIDC discovery document and JWKS of an upstream Kubernetes apiserver so Fasit
// can validate projected service account tokens minted by that cluster.
var allowedPaths = map[string]bool{
	"/.well-known/openid-configuration": true,
	"/openid/v1/jwks":                   true,
}

func newHandler(routes []route, log *slog.Logger) (http.Handler, error) {
	proxies := make(map[string]*httputil.ReverseProxy, len(routes))
	for _, r := range routes {
		target := &url.URL{Scheme: "https", Host: r.Upstream}
		upstream := r.Upstream
		rp := &httputil.ReverseProxy{
			Rewrite: func(pr *httputil.ProxyRequest) {
				pr.SetURL(target)
				pr.Out.Host = upstream
			},
			ErrorLog: slog.NewLogLogger(log.Handler(), slog.LevelError),
		}
		proxies[r.Host] = rp
		log.With("host", r.Host, "upstream", r.Upstream).Info("registered route")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !allowedPaths[r.URL.Path] {
			http.NotFound(w, r)
			return
		}
		rp, ok := proxies[r.Host]
		if !ok {
			http.Error(w, "unknown host", http.StatusNotFound)
			return
		}
		rp.ServeHTTP(w, r)
	})
	return mux, nil
}

func newLogger(level string) *slog.Logger {
	lvl := new(slog.LevelVar)
	switch level {
	case "debug":
		lvl.Set(slog.LevelDebug)
	case "warn", "warning":
		lvl.Set(slog.LevelWarn)
	case "error":
		lvl.Set(slog.LevelError)
	default:
		lvl.Set(slog.LevelInfo)
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
}
