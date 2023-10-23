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
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph/model"
)

type Store interface {
	database.EnvironmentRepo
	database.FeatureDataRepo
	database.FeatureStateRepo
	database.RolloutRepo

	database.Transaction
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

	repo Store

	// AllowAll will allow all rollout requests when set to true
	AllowAll bool
}

type Request struct {
	Chart   string `json:"chart"`
	Version string `json:"version"`
}

func New(ctx context.Context, repo Store) (*Rollout, error) {
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
		http.Error(w, "Unable to decode JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}

	feat, err := model.FromChart(body.Chart, body.Version)
	if err != nil {
		http.Error(w, "Unable to convert oci chart: "+err.Error(), http.StatusBadRequest)
		return
	}

	if feat.EnvironmentKinds == nil || len(feat.EnvironmentKinds) == 0 {
		http.Error(w, "No environments defined in Feature.yaml", http.StatusBadRequest)
		return
	}

	if feat.Source == "" {
		http.Error(w, "No source url found in Chart.yaml", http.StatusBadRequest)
		return
	}

	envNotAvailable := []model.EnvironmentKind{}
	for _, env := range feat.EnvironmentKinds {
		e, err := r.repo.EnvironmentCI(ctx, env)
		if err != nil {
			envNotAvailable = append(envNotAvailable, env)
			continue
		}

		fs, err := r.repo.FeatureStateGet(ctx, e.ID, feat.Name)
		if err != nil {
			envNotAvailable = append(envNotAvailable, env)
			continue
		}

		if !fs.Enabled {
			envNotAvailable = append(envNotAvailable, env)
			continue
		}
	}

	if _, err := r.repo.RolloutByName(ctx, feat.Name); err == nil {
		http.Error(w, "Rollout with this feature name is already in progress", http.StatusConflict)
		return
	}

	// if len(envNotAvailable) >= len(feature.EnvironmentKinds) {
	// 	w.WriteHeader(http.StatusBadRequest)
	// 	_ = json.NewEncoder(w).Encode(map[string]interface{}{
	// 		"error": fmt.Sprintf("no available environments to test in for kind(s): %v", envNotAvailable),
	// 	})
	// 	return
	// }

	details, err := feature.ParseTemplateDetails(feat.FeatureYAML.Values)
	if err != nil {
		http.Error(w, "Unable to parse feature template details: "+err.Error(), http.StatusBadRequest)
		return
	}

	id := ""

	err = r.repo.TxFunc(ctx, func(repo database.Repo) error {
		if err := repo.FeatureDataCreate(ctx, *feat, details); err != nil {
			return fmt.Errorf("feature data: %w", err)
		}

		if r, err := repo.RolloutCreate(ctx, feat.Name, body.Version); err != nil {
			return fmt.Errorf("rollout: %w", err)
		} else {
			id = r.ID.String()
		}

		return nil
	})

	if err != nil {
		http.Error(w, "unable to create rollout: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"id":              id,
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
