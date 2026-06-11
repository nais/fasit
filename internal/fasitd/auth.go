package fasitd

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/nais/fasit/internal/contextloader"
	envpkg "github.com/nais/fasit/internal/environment"
	"google.golang.org/api/idtoken"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
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
	cacheKey := issuer + "\x00" + discoveryURL
	if p, ok := providerCache.Load(cacheKey); ok {
		return p.(*oidc.Provider).Verifier(&oidc.Config{ClientID: audience}), nil
	}

	// When discovery is served via a proxy, the URL we fetch from differs from
	// the issuer claim embedded in the token. InsecureIssuerURLContext keeps the
	// expected issuer for validation while allowing discovery from discoveryURL.
	discoverFrom := issuer
	if discoveryURL != "" {
		ctx = oidc.InsecureIssuerURLContext(ctx, issuer)
		discoverFrom = discoveryURL
	}

	provider, err := oidc.NewProvider(ctx, discoverFrom)
	if err != nil {
		return nil, fmt.Errorf("create oidc provider: %w", err)
	}
	providerCache.Store(cacheKey, provider)
	return provider.Verifier(&oidc.Config{ClientID: audience}), nil
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
	)
	protogen.RegisterFasitdServer(s, srv)
	return s, nil
}
