package rollout

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-chi/chi/v5"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph/model"
)

type store interface {
	database.RolloutRepo
	database.EnvironmentRepo
	database.FeatureStateRepo
}

type Claims struct {
	Repository string `json:"repository"`
	Owner      string `json:"repository_owner"`
}

type Rollout struct {
	provder  *oidc.Provider
	verifier *oidc.IDTokenVerifier

	featureMgr *feature.Manager
	repo       store

	// AllowAll will allow all rollout requests when set to true
	AllowAll bool
}

func New(ctx context.Context, featureMgr *feature.Manager, repo store) (*Rollout, error) {
	provider, err := oidc.NewProvider(ctx, "https://token.actions.githubusercontent.com")
	if err != nil {
		return nil, err
	}

	verifier := provider.Verifier(&oidc.Config{
		SkipClientIDCheck: true,
	})

	return &Rollout{
		provder:    provider,
		verifier:   verifier,
		featureMgr: featureMgr,
		repo:       repo,
	}, nil
}

func (r *Rollout) Rollout(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	feature := r.featureMgr.Get(chi.URLParam(req, "feature"))
	if feature == nil {
		http.Error(w, "feature not found", http.StatusNotFound)
		return
	}

	if !r.validateToken(w, req, feature) {
		return
	}

	spec, err := r.createAndValidateSpec(req.Body, feature)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
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

	if len(envNotAvailable) > 0 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("feature not available in any environments: %v", envNotAvailable),
		})
		return
	}

	summaryID, err := r.repo.RolloutSummaryCreate(ctx, feature.Name)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}

	for _, env := range feature.EnvironmentKinds {
		rollout := &model.Rollout{
			RolloutSummaryID: summaryID,
			EnvironmentKind:  env,
			Feature:          feature.Name,
			Changeset: &model.RolloutChangeset{
				New: spec,
			},
		}

		_, err := r.repo.RolloutCreate(ctx, rollout)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"rollout":         summaryID,
		"envNotAvailable": envNotAvailable,
	})
}

func (r *Rollout) validateToken(w http.ResponseWriter, req *http.Request, feature *feature.Feature) (ok bool) {
	if r.AllowAll {
		return true
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

	for _, rf := range feature.RolloutSource {
		if rf.String() == claims.Repository {
			return true
		}
	}
	http.Error(w, "invalid repository", http.StatusUnauthorized)
	return false
}

func (r *Rollout) createAndValidateSpec(rd io.Reader, feature *feature.Feature) (map[string]json.RawMessage, error) {
	v := map[string]any{}
	if err := json.NewDecoder(rd).Decode(&v); err != nil {
		return nil, err
	}

	spec, ok := createRolloutSpec(v)
	if !ok {
		return nil, fmt.Errorf("invalid rollout spec, expected only changes to 'imageTag' and/or 'tag'")
	}

	changedKeys := map[string]struct{}{}
	for k := range spec {
		changedKeys[k] = struct{}{}
	}

	for k := range feature.Config {
		delete(changedKeys, k)
	}

	if len(changedKeys) > 0 {
		return nil, fmt.Errorf("changes to non-config value: %v", changedKeys)
	}

	return spec, nil
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

func createRolloutSpec(v map[string]any) (map[string]json.RawMessage, bool) {
	out := map[string]json.RawMessage{}
	if flattenSpec(out, v, "") {
		return out, true
	}
	return nil, false
}

func flattenSpec(out map[string]json.RawMessage, v map[string]any, parent string) (ok bool) {
	if len(v) == 0 {
		return false
	}

	for key, val := range v {
		key := strings.ReplaceAll(key, ".", "\\.")
		switch val := val.(type) {
		case map[string]any:
			if !flattenSpec(out, val, parent+key+".") {
				return false
			}
		case string:
			switch key {
			case "imageTag", "tag":
				// ok
			default:
				return false
			}
			out[parent+key] = json.RawMessage(strconv.Quote(val))
		default:
			return false
		}
	}
	return true
}
