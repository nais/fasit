package fasitd

import (
	"context"
	"fmt"
	"time"

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

// NewIAPStreamInterceptor validates the Google IAP JWT assertion present in the
// stream's gRPC metadata. When audience is empty (local/dev with the proxy
// skipped) validation is bypassed.
func NewIAPStreamInterceptor(audience string, insecureSkipProxy bool) grpc.StreamServerInterceptor {
	return iapStreamInterceptor(audience, insecureSkipProxy, idtoken.Validate)
}

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

// NewGrpcServer builds the dedicated, IAP-protected gRPC server that serves the
// fasitd session protocol on its own listener.
func NewGrpcServer(srv *Server, audience string, insecureSkipProxy bool) (*grpc.Server, error) {
	if !insecureSkipProxy && audience == "" {
		return nil, fmt.Errorf("INSECURE_SKIP_PROXY must be true when IAP audience is empty")
	}
	s := grpc.NewServer(
		grpc.ChainStreamInterceptor(NewIAPStreamInterceptor(audience, insecureSkipProxy)),
	)
	protogen.RegisterFasitdServer(s, srv)
	return s, nil
}
