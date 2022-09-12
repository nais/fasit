package rollout

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v4"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph/model"
)

type Claims struct {
	Repository string `json:"repository"`
	Owner      string `json:"repository_owner"`
}

type Rollout struct {
	provder     *oidc.Provider
	verifier    *oidc.IDTokenVerifier
	signkingKey any
	featureMgr  *feature.Manager
	repo        database.RolloutRepo

	// AllowAll will allow all rollout requests when set to true
	AllowAll bool
}

func New(ctx context.Context, featureMgr *feature.Manager, repo database.RolloutRepo) (*Rollout, error) {
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

func (r *Rollout) TokenExchange(w http.ResponseWriter, req *http.Request) {
	token, err := getAuthToken(req)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"error": err.Error(),
		})
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

	ret := jwt.NewWithClaims(&jwt.SigningMethodHMAC{}, jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(jwt.TimeFunc()),
		ExpiresAt: jwt.NewNumericDate(jwt.TimeFunc().Add(5 * time.Minute)),
		Subject:   claims.Repository,
	})

	retToken, err := ret.SignedString(r.signkingKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"token": retToken})
}

func (r *Rollout) Rollout(w http.ResponseWriter, req *http.Request) {
	feature := r.featureMgr.Get(chi.URLParam(req, "feature"))
	if feature == nil {
		http.Error(w, "feature not found", http.StatusNotFound)
		return
	}

	if !r.validateToken(w, req, feature) {
		return
	}

	v := map[string]any{}
	if err := json.NewDecoder(req.Body).Decode(&v); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	spec, ok := createRolloutSpec(v)
	if !ok {
		http.Error(w, "invalid rollout spec", http.StatusBadRequest)
		return
	}

	changedKeys := map[string]struct{}{}
	for k := range spec {
		changedKeys[k] = struct{}{}
	}

	for k := range feature.Config {
		delete(changedKeys, k)
	}

	if len(changedKeys) > 0 {
		http.Error(w, fmt.Sprintf("changes to non-config value: %v", changedKeys), http.StatusBadRequest)
		return
	}

	rollout := &model.Rollout{
		Feature: feature.Name,
		Changeset: &model.RolloutChangeset{
			New: spec,
		},
	}

	nw, err := r.repo.RolloutCreate(req.Context(), rollout)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]any{
		"rollout": nw.ID,
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

	tok, err := jwt.ParseWithClaims(token, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		return r.signkingKey, nil
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	claims := tok.Claims.(*jwt.RegisteredClaims)
	for _, rf := range feature.RolloutSource {
		if rf.String() == claims.Subject {
			return true
		}
	}
	return false
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
