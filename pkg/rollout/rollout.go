package rollout

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/nais/fasit/pkg/auth"
	"github.com/nais/fasit/pkg/database"
	feature "github.com/nais/fasit/pkg/feature2"
	"github.com/nais/fasit/pkg/graph/model"
)

type store interface {
	database.EnvironmentRepo
	database.FeatureStateRepo
	database.RolloutRepo
}

type Claims struct {
	Repository string `json:"repository"`
	Owner      string `json:"repository_owner"`
	Actor      string `json:"actor"`
	RunID      string `json:"run_id"`
}

type Rollout struct {
	provder  *oidc.Provider
	verifier *oidc.IDTokenVerifier

	repo store

	// AllowAll will allow all rollout requests when set to true
	AllowAll bool
}

type Request struct {
	Chart   string `json:"chart"`
	Version string `json:"version"`
}

func New(ctx context.Context, repo store) (*Rollout, error) {
	provider, err := oidc.NewProvider(ctx, "https://token.actions.githubusercontent.com")
	if err != nil {
		return nil, err
	}

	verifier := provider.Verifier(&oidc.Config{
		SkipClientIDCheck: true,
	})

	return &Rollout{
		provder:  provider,
		verifier: verifier,
		repo:     repo,
	}, nil
}

func (r *Rollout) Rollout(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	actor, valid := r.validateToken(w, req)
	if !valid {
		return
	}

	ctx = auth.SetEmail(ctx, actor)

	body := &Request{}
	err := json.NewDecoder(req.Body).Decode(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	feature, err := feature.FromChart(body.Chart, body.Version)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	envNotAvailable := []model.EnvironmentKind{}
	for _, env := range feature.EnvironmentKinds {
		e, err := r.repo.EnvironmentCI(ctx, env)
		if err != nil {
			envNotAvailable = append(envNotAvailable, env)
			continue
		}

		fs, err := r.repo.FeatureStateGet(ctx, e.ID, feature.Name)
		if err != nil {
			envNotAvailable = append(envNotAvailable, env)
			continue
		}

		if !fs.Enabled {
			envNotAvailable = append(envNotAvailable, env)
			continue
		}
	}

	if len(envNotAvailable) >= len(feature.EnvironmentKinds) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("no available environments to test in for kind(s): %v", envNotAvailable),
		})
		return
	}

	r.repo.RolloutCreate(ctx, feature.Name, body.Chart, body.Version)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"envNotAvailable": envNotAvailable,
	})
}

func (r *Rollout) validateToken(w http.ResponseWriter, req *http.Request) (actor string, ok bool) {
	if r.AllowAll {
		return "mockrollout", true
	}

	token, err := getAuthToken(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	idToken, err := r.verifier.Verify(req.Context(), token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	claims := &Claims{}
	if err := idToken.Claims(claims); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if claims.Owner != "nais" {
		http.Error(w, "invalid repository owner", http.StatusUnauthorized)
		return
	}

	return claims.Actor + "@" + claims.Repository + "/" + claims.RunID, true
}

func getAuthToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", fmt.Errorf("no authorization header")
	}

	bearer, token, ok := strings.Cut(authHeader, " ")
	if !ok || bearer != "Bearer" {
		return "", fmt.Errorf("invalid authorization header")
	}

	return token, nil
}
