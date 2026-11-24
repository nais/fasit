package fasitd

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAuthorizeRegister(t *testing.T) {
	tests := map[string]struct {
		identity *streamIdentity
		tenant   string
		env      string
		wantCode codes.Code
	}{
		"no verified identity is denied": {
			tenant: "nav", env: "dev", wantCode: codes.PermissionDenied,
		},
		"matching environment is allowed": {
			identity: &streamIdentity{tenant: "nav", environment: "dev"},
			tenant:   "nav", env: "dev", wantCode: codes.OK,
		},
		"other environment is denied": {
			identity: &streamIdentity{tenant: "nav", environment: "dev"},
			tenant:   "nav", env: "prod", wantCode: codes.PermissionDenied,
		},
		"other tenant is denied": {
			identity: &streamIdentity{tenant: "nav", environment: "dev"},
			tenant:   "ci-nais", env: "dev", wantCode: codes.PermissionDenied,
		},
		"unrestricted local stream is allowed": {
			identity: &streamIdentity{unrestricted: true},
			tenant:   "nav", env: "prod", wantCode: codes.OK,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if tc.identity != nil {
				ctx = context.WithValue(ctx, identityKey{}, *tc.identity)
			}
			err := authorizeRegister(ctx, tc.tenant, tc.env)
			if got := status.Code(err); got != tc.wantCode {
				t.Errorf("code = %v, want %v (err %v)", got, tc.wantCode, err)
			}
		})
	}
}
