package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"google.golang.org/api/idtoken"
)

type (
	contextKey int
	valfunc    func(ctx context.Context, idToken string, audience string) (*idtoken.Payload, error)
)

const contextEmail contextKey = 1

func InsecureValidateMW(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := r.Header.Get("X-Goog-Authenticated-User-Email")
		_, email, _ = strings.Cut(email, ":")

		ctx := context.WithValue(r.Context(), contextEmail, email)
		r = r.WithContext(ctx)

		h.ServeHTTP(w, r)
	})
}

func ValidateJWTFromComputeEngine(aud string) func(h http.Handler) http.Handler {
	return validateJWTFromComputeEngine(aud, idtoken.Validate)
}

func validateJWTFromComputeEngine(aud string, validator valfunc) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			iapJWT := r.Header.Get("X-Goog-IAP-JWT-Assertion")

			payload, err := validator(r.Context(), iapJWT, aud)
			if err != nil {
				http.Error(w, "Invalid JWT token", http.StatusUnauthorized)
				return
			}

			if time.Unix(payload.IssuedAt, 0).After(time.Now().Add(30 * time.Second)) {
				http.Error(w, "JWT token is in the future", http.StatusUnauthorized)
				return
			}

			if payload.Issuer != "https://cloud.google.com/iap" {
				http.Error(w, "Invalid JWT token issuer", http.StatusUnauthorized)
				return
			}

			email := r.Header.Get("X-Goog-Authenticated-User-Email")
			_, email, _ = strings.Cut(email, ":")

			ctx := context.WithValue(r.Context(), contextEmail, email)
			r = r.WithContext(ctx)

			h.ServeHTTP(w, r)
		})
	}
}

func GetEmail(ctx context.Context) string {
	email := ctx.Value(contextEmail)
	if email == nil {
		return "unauthorized"
	}
	return email.(string)
}
