package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/api/sqlgen"
	"github.com/nais/fasit/internal/auth"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/model"
	"github.com/nais/fasit/internal/reconciler"
)

func NewHttpHandler(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) (*HttpHandler, error) {
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
		log:            log.With("subsystem", "featureassignment-http"),
		programContext: ctx,
		querier:        sqlgen.New(pool),
	}, nil
}

func (h *HttpHandler) GetFeatureAssignment(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	if _, valid := h.validateToken(w, req); !valid {
		return
	}

	assignmentID, err := uuid.Parse(chi.URLParam(req, "id"))
	if err != nil {
		http.Error(w, "invalid assignment id", http.StatusBadRequest)
		h.log.With("err", err).Error("convert assignment ID")
		return
	}

	if _, err := h.querier.GetFeatureAssignment(ctx, assignmentID); errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "assignment does not exist", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "unable to get assignment", http.StatusInternalServerError)
		h.log.With("err", err).Error("get assignment")
		return
	}

	state := model.FeatureReconcileStatusStateUnknown
	states, err := h.listReconcileStatuses(ctx, assignmentID)
	if err != nil {
		// Degrade to UNKNOWN rather than 500: clients are expected to keep polling.
		h.log.With("err", err).Warn("list reconcile statuses; returning UNKNOWN")
	} else {
		state, _ = states.Aggregate()
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(GetFeatureAssignmentResponse{
		ID:    assignmentID,
		State: state,
	}); err != nil {
		h.log.With("err", err).Error("encode assignment response")
	}
}

func (h *HttpHandler) CreateFeatureAssignment(w http.ResponseWriter, req *http.Request) {
	actor, valid := h.validateToken(w, req)
	if !valid {
		return
	}

	// use the program context instead of the request context to avoid cancellation when the client disconnects, as
	// deployments may take a while to create
	ctx := auth.SetEmail(h.programContext, actor)

	body := CreateFeatureAssignmentRequest{}
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

	deploymentID, err := featureassignment.Create(ctx, featureassignment.CreateFeatureAssignment{
		Chart:       body.Chart,
		Version:     body.Version,
		Description: &body.Description,
		Commit:      body.Ref,
		Target:      body.Target,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		h.log.With("err", err).Error("create assignment")
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id": deploymentID.String(),
	})

	reconciler.TriggerReconcile()
}

func (h *HttpHandler) validateToken(w http.ResponseWriter, req *http.Request) (actor string, ok bool) {
	if h.AllowAll {
		return "mockassignment", true
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
