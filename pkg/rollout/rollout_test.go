package rollout

import (
	"crypto"
	"crypto/x509"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/go-cmp/cmp"
)

func TestValidRolloutSpec(t *testing.T) {
	tests := map[string]struct {
		input map[string]any
		want  map[string]json.RawMessage
		ok    bool
	}{
		"empty": {
			input: map[string]any{},
			ok:    false,
		},
		"invalid key": {
			input: map[string]any{
				"invalid": "invalid",
			},
			ok: false,
		},
		"invalid value type": {
			input: map[string]any{
				"tag": 123,
			},
			ok: false,
		},
		"valid imageTag": {
			input: map[string]any{
				"imageTag": "newtag",
			},
			want: map[string]json.RawMessage{
				"imageTag": json.RawMessage(`"newtag"`),
			},
			ok: true,
		},
		"valid tag": {
			input: map[string]any{
				"tag": "newtag",
			},
			want: map[string]json.RawMessage{
				"tag": json.RawMessage(`"newtag"`),
			},
			ok: true,
		},
		"valid tag with invalid key": {
			input: map[string]any{
				"tag":     "newtag",
				"invalid": "invalid",
			},
			ok: false,
		},
		"nested valid tag with invalid value type": {
			input: map[string]any{
				"tag": "newtag",
				"nested": map[string]any{
					"tag": 123,
				},
			},
			ok: false,
		},
		"nested valid tag with valid value type": {
			input: map[string]any{
				"tag": "newtag",
				"nested": map[string]any{
					"tag": "123",
				},
			},
			want: map[string]json.RawMessage{
				"tag":        json.RawMessage(`"newtag"`),
				"nested.tag": json.RawMessage(`"123"`),
			},
			ok: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, ok := createRolloutSpec(tc.input)
			if ok != tc.ok {
				t.Errorf("got %v, want %v", ok, tc.ok)
			}

			if !cmp.Equal(tc.want, got) {
				t.Errorf("diff -want +got:\n%v", cmp.Diff(tc.want, got))
			}
		})
	}
}

func TestRollout_TokenExchange(t *testing.T) {
	oldTime := jwt.TimeFunc
	defer func() { jwt.TimeFunc = oldTime }()
	jwt.TimeFunc = func() time.Time {
		return time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	tests := map[string]struct {
		Authorization  string
		ExpectedStatus int
		ExpectedBody   string
	}{
		"no header": {
			Authorization:  "",
			ExpectedStatus: 401,
			ExpectedBody:   `{"error":"no authorization header"}`,
		},
		"invalid header": {
			Authorization:  "invalid",
			ExpectedStatus: 401,
			ExpectedBody:   `{"error":"invalid authorization header"}`,
		},
		"valid token, expired": {
			Authorization:  "Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImlhdCI6MTUxNjIzOTAyMn0.t1I9UhZ3NMMrbQFXPVW31hV40O-JUdxxs31MyWh7FH3EFEc03ZlxHANzypw2YEoCeRPZKgm_UZdnHEkJvXiI-HLfyfdmKxVCArK1eTlKF1aEhXvesU0Bhu4F1U0TaZfIPxZ1f85cJq2MbmXrZdjdosEHcdwY4UH9Kv6MToC2PwOUEr0dRmzfsgAgxJe96pFe8d4QgfJnlNXzCdJF5QryMsw-8U301Mq7QqCB51xzxzSQEXBfJMj-Jr4RSsyhNzyOQSFJ0KmbNg6w1xE8bp6_LPzxFXSXu-RYY0-veJuYY58EZPppNwOf0Ocr8Kr0gKohHLk8Ki9KgPP4u2bsNjHUgI8EQBa_AnQPoVQEzZbuNQP-0mxQF4LUUZi22Gx_38_YiOW2BorN3m0Z5g-G0R_Af972tijzIQ9C2IU3yd8XIQ9DgDDf3lfJ4DYnMqOELhCAGb0orw99aX1dnqs0yMu4n1wDX-RrQHQYE5RiHrAL7xk_5ubJFJuqwDdUl9ZAcRVJ8e8GFk3s3tZNBZDR45eB_VoIUBjAsFn9izIwBUJMYE0EF-2LsHXdSlWK8gUHEDKr0dDRoZcnOR-DkkYSjU8LHKSsXIMZSoyhTCVlptOZVNkcwM8I8AfVyLprjIWwrOBKDIS4ZvaRQo2-2ombDOMGjpqRHmQQXuPI9v35uF83Qjo",
			ExpectedStatus: 401,
			ExpectedBody:   "",
		},
	}

	b, err := os.ReadFile("./testdata/jwtRS256.key.pub")
	if err != nil {
		t.Fatal(err)
	}
	// block, _ := pem.Decode(b)
	pub1, _ := x509.ParsePKCS1PublicKey(b)

	keySet := &oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{pub1}}
	verifier := oidc.NewVerifier("https://token.actions.githubusercontent.com", keySet, &oidc.Config{
		Now: jwt.TimeFunc,
	})

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			r := &Rollout{
				verifier: verifier,
			}

			req := &http.Request{
				Header: http.Header{
					"Authorization": []string{tc.Authorization},
				},
			}
			w := httptest.NewRecorder()

			r.TokenExchange(w, req)

			if w.Code != tc.ExpectedStatus {
				t.Errorf("got %v, want %v", w.Code, tc.ExpectedStatus)
			}

			want := strings.TrimSpace(w.Body.String())
			if !cmp.Equal(tc.ExpectedBody, want) {
				t.Errorf("diff -want +got:\n%v", cmp.Diff(tc.ExpectedBody, want))
			}
		})
	}
}
