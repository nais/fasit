package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"google.golang.org/api/idtoken"
)

func TestInsecureValidateMW(t *testing.T) {
	th := func(w http.ResponseWriter, r *http.Request) {
		val := r.Context().Value(contextEmail)
		fmt.Fprint(w, val)
	}

	rec := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Goog-Authenticated-User-Email", "accounts.google.com:example@gmail.com")
	InsecureValidateMW(http.HandlerFunc(th)).ServeHTTP(rec, r)
	if got := rec.Body.String(); got != "example@gmail.com" {
		t.Errorf("InsecureValidateMW() = %v, want %v", got, "example@gmail.com")
	}
}

func TestValidateJWTFromComputeEngine(t *testing.T) {
	// The following tests disables the actual token validator and just "validates" all of them as valid.
	tests := map[string]struct {
		issuer         string
		issuedAt       int64
		jwtAssertion   string
		expectedStatus int
	}{
		"unauthorized": {
			expectedStatus: http.StatusUnauthorized,
		},
		"invalid issuer": {
			issuer:         "https://example.com",
			expectedStatus: http.StatusUnauthorized,
		},
		"invalid issuedAt": {
			issuer:         "https://cloud.google.com/iap",
			issuedAt:       time.Now().Add(3 * time.Minute).Unix(), // in the future
			expectedStatus: http.StatusUnauthorized,
		},
		"valid": {
			issuer:         "https://cloud.google.com/iap",
			issuedAt:       time.Now().Unix(),
			jwtAssertion:   "valid",
			expectedStatus: http.StatusOK,
		},
	}

	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			validator := func(ctx context.Context, idToken string, audience string) (*idtoken.Payload, error) {
				return &idtoken.Payload{
					Issuer:   tt.issuer,
					IssuedAt: tt.issuedAt,
				}, nil
			}

			rec := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/", nil)
			r.Header.Set("X-Goog-IAP-JWT-Assertion", tt.jwtAssertion)
			validateJWTFromComputeEngine("aud", validator)(okHandler).ServeHTTP(rec, r)

			if got := rec.Code; got != tt.expectedStatus {
				t.Errorf("validateJWTFromComputeEngine() = %v, want %v", got, tt.expectedStatus)
			}
		})
	}
}

func TestGetEmail(t *testing.T) {
	tests := map[string]struct {
		ctx  context.Context
		want string
	}{
		"no email": {
			ctx:  context.Background(),
			want: "unauthorized",
		},
		"email": {
			ctx:  context.WithValue(context.Background(), contextEmail, "some@email.com"),
			want: "some@email.com",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := GetEmail(tt.ctx); got != tt.want {
				t.Errorf("GetEmail() = %v, want %v", got, tt.want)
			}
		})
	}
}
