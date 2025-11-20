package deployment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
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
	database.DeploymentRepo
	database.Transaction
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
	ChartURL    string             `json:"chartUrl"`
	Version     string             `json:"version"`
	Description string             `json:"description"`
	Ref         *model.GHRef       `json:"ref"`
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

	feat, err := model.FromChart(body.ChartURL, body.Version)
	if err != nil {
		http.Error(w, "Unable to convert oci chart: "+err.Error(), http.StatusBadRequest)
		return
	}

	// TODO: if we remove kind as a concept we need to change featureData table and logic
	if len(feat.EnvironmentKinds) == 0 {
		http.Error(w, "No environments defined in Feature.yaml", http.StatusBadRequest)
		return
	}

	if feat.Source == "" {
		http.Error(w, "No source url found in Chart.yaml", http.StatusBadRequest)
		return
	}

	details, err := feature.ParseTemplateDetails(feat.FeatureYAML.Values)
	if err != nil {
		http.Error(w, "Unable to parse feature template details: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := d.repo.FeatureDataCreate(ctx, *feat, details); err != nil {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
			http.Error(w, "unable to create feature data", http.StatusInternalServerError)
			d.log.WithError(err).Error("create feature data")
			return
		}
	}

	if ref := body.Ref; ref != nil {
		// Make sure all fields are set
		if ref.Owner == "" || ref.Repo == "" || ref.Ref == "" {
			http.Error(w, "invalid ref, missing owner, repo or ref", http.StatusBadRequest)
			return
		}
	}

	deployment, err := d.repo.V3DeploymentCreate(ctx, feat.Name, body.Version, body.Ref, body.Target)
	if err != nil {
		http.Error(w, "unable to create deployment", http.StatusInternalServerError)
		d.log.WithError(err).Error("create deployment")
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id": deployment.ID.String(),
	})

	go func() {
		select {
		case d.reconcileTrigger <- ReconcileTriggerEvent{
			DeploymentID:   deployment.ID,
			FeatureName:    feat.Name,
			FeatureVersion: body.Version,
			Type:           ReconcileTriggerEventTypeNewDeployment,
		}:
		default:
			d.log.Debug("there is already a reconcile event queued, skipping")
		}
	}()
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
