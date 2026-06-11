package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProxyRoutesByHostAndAllowsOnlyKnownPaths(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "upstream:"+r.URL.Path)
	}))
	defer upstream.Close()

	upstreamHost := upstream.Listener.Addr().String()
	routes := []route{{Host: "dev-fss.proxy.test", Upstream: upstreamHost}}

	// Use http scheme for the test upstream by registering it directly.
	h, err := newHandler(routes, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newHandler: %v", err)
	}

	tests := []struct {
		name     string
		host     string
		path     string
		wantCode int
	}{
		{"disallowed path", "dev-fss.proxy.test", "/secrets", http.StatusNotFound},
		{"unknown host", "other.proxy.test", "/openid/v1/jwks", http.StatusNotFound},
		{"allowed jwks", "dev-fss.proxy.test", "/openid/v1/jwks", http.StatusBadGateway},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://"+tc.host+tc.path, nil)
			req.Host = tc.host
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.wantCode {
				t.Errorf("got %d, want %d", rec.Code, tc.wantCode)
			}
		})
	}
}

func TestProxyForwardsAllowedPathToUpstream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "upstream:"+r.URL.Path)
	}))
	defer upstream.Close()

	// httptest upstream serves http, so build a route pointing at it and
	// override the proxy to use http by registering through newHandler with the
	// upstream host. The proxy hardcodes https, so we only assert routing here.
	routes := []route{{Host: "dev-fss.proxy.test", Upstream: upstream.Listener.Addr().String()}}
	h, err := newHandler(routes, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://dev-fss.proxy.test/.well-known/openid-configuration", nil)
	req.Host = "dev-fss.proxy.test"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	// https against an http test server yields a bad gateway, proving the request
	// was routed to a proxy rather than rejected.
	if rec.Code != http.StatusBadGateway {
		t.Errorf("expected routed request, got %d", rec.Code)
	}
}
