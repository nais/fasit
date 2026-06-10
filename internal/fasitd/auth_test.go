package fasitd

import (
	"context"
	"testing"
	"time"

	"google.golang.org/api/idtoken"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type fakeStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (f fakeStream) Context() context.Context { return f.ctx }

func okValidator(_ context.Context, _ string, _ string) (*idtoken.Payload, error) {
	return &idtoken.Payload{
		Issuer:   "https://cloud.google.com/iap",
		IssuedAt: time.Now().Add(-time.Minute).Unix(),
	}, nil
}

func runInterceptor(t *testing.T, audience string, insecure bool, ctx context.Context) error {
	t.Helper()
	called := false
	interceptor := iapStreamInterceptor(audience, insecure, okValidator)
	err := interceptor(nil, fakeStream{ctx: ctx}, &grpc.StreamServerInfo{}, func(any, grpc.ServerStream) error {
		called = true
		return nil
	})
	if err == nil && !called {
		t.Errorf("handler not called despite nil error")
	}
	return err
}

func TestIAPInterceptorInsecureBypass(t *testing.T) {
	if err := runInterceptor(t, "", true, context.Background()); err != nil {
		t.Errorf("insecure should bypass auth, got %v", err)
	}
}

func TestIAPInterceptorMissingAssertion(t *testing.T) {
	if err := runInterceptor(t, "aud", false, context.Background()); err == nil {
		t.Errorf("expected error when IAP assertion is missing")
	}
}

func TestIAPInterceptorValidAssertion(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(iapAssertionHeader, "token"))
	if err := runInterceptor(t, "aud", false, ctx); err != nil {
		t.Errorf("valid assertion should pass, got %v", err)
	}
}

func TestIAPInterceptorMissingAudience(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(iapAssertionHeader, "token"))
	if err := runInterceptor(t, "", false, ctx); err == nil {
		t.Errorf("expected error when audience is not configured and proxy not skipped")
	}
}
