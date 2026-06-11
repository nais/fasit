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

// NewIAPStreamInterceptor validates the Google IAP JWT assertion or KSA OIDC token present in the
// stream's gRPC metadata. When audience is empty (local/dev with the proxy
// skipped) validation is bypassed.
func NewIAPStreamInterceptor(loadContext contextloader.LoaderFunc, audience string, insecureSkipProxy bool) grpc.StreamServerInterceptor {
	return iapStreamInterceptor(loadContext, audience, insecureSkipProxy, idtoken.Validate)
}

// TODO: Cache OIDC providers instead of recreating them per request.
var providerCache sync.Map

func getOIDCVerifier(ctx context.Context, issuer, audience string) (*oidc.IDTokenVerifier, error) {
	// Simple caching (TODO: proper concurrency and expiration)
	if p, ok := providerCache.Load(issuer); ok {
		return p.(*oidc.Provider).Verifier(&oidc.Config{ClientID: audience}), nil
	}
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("create oidc provider: %w", err)
	}
	providerCache.Store(issuer, provider)
	return provider.Verifier(&oidc.Config{ClientID: audience}), nil
}

func iapStreamInterceptor(loadContext contextloader.LoaderFunc, audience string, insecureSkipProxy bool, validate jwtValidator) grpc.StreamServerInterceptor {
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

		// If a standard Authorization Bearer token is provided, it's a KSA token
		authHeaders := md.Get("authorization")
		if len(authHeaders) > 0 && strings.HasPrefix(strings.ToLower(authHeaders[0]), "bearer ") {
			rawToken := authHeaders[0][7:]
			
			tenantHeaders := md.Get("x-fasit-tenant")
			envHeaders := md.Get("x-fasit-environment")
			if len(tenantHeaders) == 0 || len(envHeaders) == 0 {
				return status.Error(codes.InvalidArgument, "missing tenant/environment headers for KSA auth")
			}
			tenantName, envName := tenantHeaders[0], envHeaders[0]

			reqCtx := loadContext(ctx)
			tenant, err := envpkg.GetTenantByName(reqCtx, tenantName)
			if err != nil {
				return status.Errorf(codes.NotFound, "unknown tenant: %v", err)
			}
			env, err := envpkg.GetByName(reqCtx, tenant.ID, envName)
			if err != nil {
				return status.Errorf(codes.NotFound, "unknown environment: %v", err)
			}

			issuerVal, err := envpkg.GetEnvironmentValue(reqCtx, env.ID, "apiserver_oidc_issuer", false)
			if err != nil {
				return status.Errorf(codes.FailedPrecondition, "missing apiserver_oidc_issuer for environment: %v", err)
			}

			verifier, err := getOIDCVerifier(reqCtx, string(issuerVal.Value), audience)
			if err != nil {
				return status.Errorf(codes.Internal, "configure oidc verifier: %v", err)
			}

			if _, err := verifier.Verify(reqCtx, rawToken); err != nil {
				return status.Errorf(codes.Unauthenticated, "invalid KSA token: %v", err)
			}
			
			return handler(srv, ss)
		}

		if err := validateIAP(ctx, md, audience, validate); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}

func validateIAP(ctx context.Context, md metadata.MD, audience string, validate jwtValidator) error {
	vals := md.Get(iapAssertionHeader)
	if len(vals) == 0 || vals[0] == "" {
		return status.Error(codes.Unauthenticated, "missing IAP assertion or authorization header")
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

// NewGrpcServer builds the dedicated, IAP-protected gRPC server that serves the
// fasitd session protocol on its own listener.
func NewGrpcServer(srv *Server, loadContext contextloader.LoaderFunc, audience string, insecureSkipProxy bool) (*grpc.Server, error) {
	if !insecureSkipProxy && audience == "" {
		return nil, fmt.Errorf("INSECURE_SKIP_PROXY must be true when IAP audience is empty")
	}
	s := grpc.NewServer(
		grpc.ChainStreamInterceptor(NewIAPStreamInterceptor(loadContext, audience, insecureSkipProxy)),
	)
	protogen.RegisterFasitdServer(s, srv)
	return s, nil
}
