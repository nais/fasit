package deployment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/internal/auth"
	"github.com/sirupsen/logrus"
)

type HttpHandler struct {
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	log      logrus.FieldLogger
	// AllowAll will allow all rollout requests when set to true
	AllowAll bool

	programContext context.Context
}

type Claims struct {
	Repository string `json:"repository"`
	Owner      string `json:"repository_owner"`
	Actor      string `json:"actor"`
	RunID      string `json:"run_id"`
}

func NewHttpHandler(ctx context.Context, log logrus.FieldLogger) (*HttpHandler, error) {
	provider, err := oidc.NewProvider(ctx, "https://token.actions.githubusercontent.com")
	if err != nil {
		return nil, err
	}

	verifier := provider.Verifier(&oidc.Config{
		SkipClientIDCheck: true,
	})

	return &HttpHandler{
		provider:       provider,
		verifier:       verifier,
		log:            log.WithField("subsystem", "deployment-http"),
		programContext: ctx,
	}, nil
}

func (h *HttpHandler) GetDeployment(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	if _, valid := h.validateToken(w, req); !valid {
		return
	}

	deploymentID, err := uuid.Parse(chi.URLParam(req, "id"))
	if err != nil {
		http.Error(w, "invalid deployment id", http.StatusBadRequest)
		h.log.WithError(err).Error("convert deployment ID")
		return
	}

	deployment, err := GetDeployment(ctx, deploymentID)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "deployment does not exist", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "unable to get deployment", http.StatusInternalServerError)
		h.log.WithError(err).Error("get deployment")
		return
	}

	_ = json.NewEncoder(w).Encode(deployment)
}

func (h *HttpHandler) CreateDeployment(w http.ResponseWriter, req *http.Request) {
	actor, valid := h.validateToken(w, req)
	if !valid {
		return
	}

	// use the program context instead of the request context to avoid cancellation when the client disconnects, as
	// deployments may take a while to create
	ctx := auth.SetEmail(h.programContext, actor)

	body := Request{}
	err := json.NewDecoder(req.Body).Decode(&body)
	if err != nil {
		http.Error(w, "Unable to decode JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if ref := body.Ref; ref != nil {
		if ref.Owner == "" || ref.Repo == "" || ref.Ref == "" {
			http.Error(w, "invalid ref, missing owner, repo or ref", http.StatusBadRequest)
			return
		}
	}

	deploymentID, err := CreateDeployment(ctx, body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		h.log.WithError(err).Error("create deployment")
		return
	}

	if body.CI.Wait {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": deploymentID.String(),
		})
	} else {
		w.WriteHeader(http.StatusAccepted)
	}

	TriggerReconcile(ctx, ReconcileTriggerEvent{})
}

func (h *HttpHandler) validateToken(w http.ResponseWriter, req *http.Request) (actor string, ok bool) {
	if h.AllowAll {
		return "mockdeployment", true
	}

	token, err := getAuthToken(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	idToken, err := h.verifier.Verify(req.Context(), token)
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
