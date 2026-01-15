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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/nais/fasit/internal/auth"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/sirupsen/logrus"
)

type Store interface {
	database.FeatureDataRepo
	database.FeaturesRepo
	database.DeploymentRepo
	database.Transaction
}

type HttpError struct {
	Message string
	Code    int
	err     error
}

func (e *HttpError) Error() string {
	msg := e.Message
	if e.err != nil {
		msg += ": " + e.err.Error()
	}
	return msg
}

func (e *HttpError) Unwrap() error {
	return e.err
}

func CreateDeployment(ctx context.Context, repo Store, req *Request) (uuid.UUID, error) {
	feat, err := model.FromChart(req.Chart, req.Version)
	if err != nil {
		return uuid.Nil, &HttpError{err: err, Message: "Unable to convert oci chart", Code: http.StatusBadRequest}
	}

	// TODO: if we remove kind as a concept we need to change featureData table and logic
	if len(feat.EnvironmentKinds) == 0 {
		return uuid.Nil, &HttpError{err: err, Message: "No environments defined in Feature.yaml", Code: http.StatusBadRequest}
	}

	if feat.Source == "" {
		return uuid.Nil, &HttpError{err: err, Message: "No source url found in Chart.yaml", Code: http.StatusBadRequest}
	}

	details, err := feature.ParseTemplateDetails(feat.FeatureYAML.Values)
	if err != nil {
		return uuid.Nil, &HttpError{err: err, Message: "Unable to parse feature template details", Code: http.StatusBadRequest}
	}

	if err := repo.FeatureDataCreate(ctx, *feat, details); err != nil {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
			return uuid.Nil, &HttpError{err: err, Message: "Unable to create feature data", Code: http.StatusInternalServerError}
		}
	}

	deployment, err := repo.V3DeploymentCreate(ctx, feat.Name, req.Version, req.Description, req.Ref, req.Target)
	if err != nil {
		return uuid.Nil, &HttpError{err: err, Message: "Unable to create deployment", Code: http.StatusInternalServerError}
	}

	if req.Global {
		if err := repo.FeatureVersionUpdate(ctx, feat.Name, req.Version); err != nil {
			return uuid.Nil, &HttpError{err: err, Message: "Unable to update feature version", Code: http.StatusInternalServerError}
		}
	}

	return deployment.ID, nil
}

type Deployment struct {
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier

	repo Store

	log logrus.FieldLogger
	// AllowAll will allow all rollout requests when set to true
	AllowAll bool

	reconcileTrigger chan<- ReconcileTriggerEvent
}

type Claims struct {
	Repository string `json:"repository"`
	Owner      string `json:"repository_owner"`
	Actor      string `json:"actor"`
	RunID      string `json:"run_id"`
}

type Request struct {
	Chart       string             `json:"chart"`
	Version     string             `json:"version"`
	Description string             `json:"description"`
	Ref         *model.GHRef       `json:"ref"`
	Global      bool               `json:"global"`
	Target      environment.Labels `json:"target"`
}

func New(ctx context.Context, repo database.Repo, reconcileTrigger chan<- ReconcileTriggerEvent, log logrus.FieldLogger) (*Deployment, error) {
	provider, err := oidc.NewProvider(ctx, "https://token.actions.githubusercontent.com")
	if err != nil {
		return nil, err
	}

	verifier := provider.Verifier(&oidc.Config{
		SkipClientIDCheck: true,
	})

	return &Deployment{
		provider:         provider,
		verifier:         verifier,
		repo:             repo,
		log:              log,
		reconcileTrigger: reconcileTrigger,
	}, nil
}

func (d *Deployment) GetDeployment(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	if _, valid := d.validateToken(w, req); !valid {
		return
	}

	deploymentID, err := uuid.Parse(chi.URLParam(req, "id"))
	if err != nil {
		http.Error(w, "invalid deployment id", http.StatusBadRequest)
		d.log.WithError(err).Error("convert deployment ID")
		return
	}

	deployment, err := d.repo.V3DeploymentGet(ctx, deploymentID)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "deployment does not exist", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "unable to get deployment", http.StatusInternalServerError)
		d.log.WithError(err).Error("get deployment")
		return
	}

	_ = json.NewEncoder(w).Encode(deployment)
}

func (d *Deployment) CreateDeployment(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	actor, valid := d.validateToken(w, req)
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

	if ref := body.Ref; ref != nil {
		if ref.Owner == "" || ref.Repo == "" || ref.Ref == "" {
			http.Error(w, "invalid ref, missing owner, repo or ref", http.StatusBadRequest)
			return
		}
	}

	deploymentID, err := CreateDeployment(ctx, d.repo, body)
	if err != nil {
		var httpErr *HttpError
		if errors.As(err, &httpErr) {
			http.Error(w, httpErr.Error(), httpErr.Code)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		d.log.WithError(err).Error("create deployment")
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id": deploymentID.String(),
	})

	TriggerReconcile(ReconcileTriggerEvent{}, d.reconcileTrigger, d.log)
}

func (d *Deployment) validateToken(w http.ResponseWriter, req *http.Request) (actor string, ok bool) {
	if d.AllowAll {
		return "mockdeployment", true
	}

	token, err := getAuthToken(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	idToken, err := d.verifier.Verify(req.Context(), token)
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
