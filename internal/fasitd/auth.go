package fasitd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/nais/fasit/internal/contextloader"
	envpkg "github.com/nais/fasit/internal/environment"
	"google.golang.org/api/idtoken"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/nais/fasit/internal/fasitd/protogen"
)

// iapAssertionHeader is the gRPC metadata key Google IAP injects the signed JWT
// assertion under. It is a header name, not a secret.
const iapAssertionHeader = "x-goog-iap-jwt-assertion" // #nosec G101

type jwtValidator func(ctx context.Context, idToken, audience string) (*idtoken.Payload, error)

// NewStreamInterceptor authenticates incoming fasitd streams. Agents exposed to
// the internet send a projected KSA token in the Authorization header along
// with x-fasit-tenant / x-fasit-environment, validated against the cluster's
// OIDC issuer. Otherwise it falls back to the Google IAP JWT assertion. When
// audience is empty (local/dev with the proxy skipped) validation is bypassed.
func NewStreamInterceptor(loadContext contextloader.LoaderFunc, audience string, insecureSkipProxy bool) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if insecureSkipProxy {
			return handler(srv, ss)
		}
		if audience == "" {
			return status.Error(codes.Internal, "IAP audience is not configured")
		}

		ctx := ss.Context()
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return status.Error(codes.Unauthenticated, "missing metadata")
		}

		authHeaders := md.Get("authorization")
		if len(authHeaders) > 0 && strings.HasPrefix(strings.ToLower(authHeaders[0]), "bearer ") {
			rawToken := authHeaders[0][len("bearer "):]
			if err := validateKSAToken(ctx, loadContext, md, rawToken, audience); err != nil {
				return err
			}
			return handler(srv, ss)
		}

		if err := validateIAP(ctx, audience, idtoken.Validate); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}

// TODO: replace this naive map with a provider cache that handles refresh and
// eviction; providers are created once per issuer here and never invalidated.
var providerCache sync.Map

func getOIDCVerifier(ctx context.Context, issuer, discoveryURL, audience string) (*oidc.IDTokenVerifier, error) {
	cfg := &oidc.Config{ClientID: audience}

	// No explicit discovery URL: let go-oidc discover from the issuer. It appends
	// /.well-known/openid-configuration to the issuer itself.
	if discoveryURL == "" {
		if p, ok := providerCache.Load(issuer); ok {
			return p.(*oidc.Provider).Verifier(cfg), nil
		}
		provider, err := oidc.NewProvider(ctx, issuer)
		if err != nil {
			return nil, fmt.Errorf("discover from issuer: %w", err)
		}
		providerCache.Store(issuer, provider)
		return provider.Verifier(cfg), nil
	}

	// Explicit discovery URL: it is the full discovery-document endpoint (e.g.
	// served via oidcproxy), fetched verbatim. The token's iss is still validated
	// against the configured issuer, which may differ from where we fetch keys.
	if ks, ok := keySetCache.Load(discoveryURL); ok {
		return oidc.NewVerifier(issuer, ks.(oidc.KeySet), cfg), nil
	}
	keySet, err := remoteKeySetFromDiscovery(ctx, discoveryURL)
	if err != nil {
		return nil, err
	}
	keySetCache.Store(discoveryURL, keySet)
	return oidc.NewVerifier(issuer, keySet, cfg), nil
}

// keySetCache holds JWKS key sets keyed by their full discovery URL. The key
// sets refresh themselves on their own background context.
var keySetCache sync.Map

func remoteKeySetFromDiscovery(ctx context.Context, discoveryURL string) (oidc.KeySet, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build discovery request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req) // #nosec G704 -- discoveryURL is the environment's admin-configured oidc_discovery_url, not request-derived
	if err != nil {
		return nil, fmt.Errorf("fetch discovery document: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("discovery document %s: status %d: %s", discoveryURL, resp.StatusCode, body)
	}

	var doc struct {
		JWKSURI string `json:"jwks_uri"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode discovery document: %w", err)
	}
	if doc.JWKSURI == "" {
		return nil, fmt.Errorf("discovery document %s has no jwks_uri", discoveryURL)
	}

	// Background context: the cached key set lives for the process lifetime and
	// must not be tied to the request that first created it.
	return oidc.NewRemoteKeySet(context.Background(), doc.JWKSURI), nil
}

func validateKSAToken(ctx context.Context, loadContext contextloader.LoaderFunc, md metadata.MD, rawToken, audience string) error {
	tenantHeaders := md.Get("x-fasit-tenant")
	envHeaders := md.Get("x-fasit-environment")
	if len(tenantHeaders) == 0 || len(envHeaders) == 0 {
		return status.Error(codes.InvalidArgument, "missing tenant/environment headers for KSA auth")
	}

	reqCtx := loadContext(ctx)
	tenant, err := envpkg.GetTenantByName(reqCtx, tenantHeaders[0])
	if err != nil {
		return status.Errorf(codes.NotFound, "unknown tenant: %v", err)
	}
	env, err := envpkg.GetByName(reqCtx, tenant.ID, envHeaders[0])
	if err != nil {
		return status.Errorf(codes.NotFound, "unknown environment: %v", err)
	}

	if env.OIDCIssuer == nil || *env.OIDCIssuer == "" {
		return status.Error(codes.FailedPrecondition, "environment has no OIDC issuer configured")
	}
	discoveryURL := ""
	if env.OIDCDiscoveryURL != nil {
		discoveryURL = *env.OIDCDiscoveryURL
	}

	verifier, err := getOIDCVerifier(reqCtx, *env.OIDCIssuer, discoveryURL, audience)
	if err != nil {
		return status.Errorf(codes.Internal, "configure oidc verifier: %v", err)
	}
	if _, err := verifier.Verify(reqCtx, rawToken); err != nil {
		return status.Errorf(codes.Unauthenticated, "invalid KSA token: %v", err)
	}
	return nil
}

// iapStreamInterceptor validates only the Google IAP JWT assertion. It is the
// pre-KSA behaviour, retained for the dedicated interceptor unit tests.
func iapStreamInterceptor(audience string, insecureSkipProxy bool, validate jwtValidator) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if insecureSkipProxy {
			return handler(srv, ss)
		}
		if audience == "" {
			return status.Error(codes.Internal, "IAP audience is not configured")
		}
		if err := validateIAP(ss.Context(), audience, validate); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}

func validateIAP(ctx context.Context, audience string, validate jwtValidator) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}
	vals := md.Get(iapAssertionHeader)
	if len(vals) == 0 || vals[0] == "" {
		return status.Error(codes.Unauthenticated, "missing IAP assertion")
	}

	payload, err := validate(ctx, vals[0], audience)
	if err != nil {
		return status.Error(codes.Unauthenticated, "invalid IAP assertion")
	}
	if payload.Issuer != "https://cloud.google.com/iap" {
		return status.Error(codes.Unauthenticated, "invalid IAP assertion issuer")
	}
	if time.Unix(payload.IssuedAt, 0).After(time.Now().Add(30 * time.Second)) {
		return status.Error(codes.Unauthenticated, "IAP assertion issued in the future")
	}
	return nil
}

// NewGrpcServer builds the dedicated, authenticated gRPC server that serves the
// fasitd session protocol on its own listener.
func NewGrpcServer(srv *Server, loadContext contextloader.LoaderFunc, audience string, insecureSkipProxy bool) (*grpc.Server, error) {
	if !insecureSkipProxy && audience == "" {
		return nil, fmt.Errorf("INSECURE_SKIP_PROXY must be true when IAP audience is empty")
	}
	s := grpc.NewServer(
		grpc.ChainStreamInterceptor(NewStreamInterceptor(loadContext, audience, insecureSkipProxy)),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    30 * time.Second,
			Timeout: 10 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	protogen.RegisterFasitdServer(s, srv)
	return s, nil
}
