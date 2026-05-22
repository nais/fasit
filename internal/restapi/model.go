package restapi

import (
	"context"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/model"
	"github.com/nais/fasit/internal/restapi/restapisql"
	"github.com/sirupsen/logrus"
)

type GetDeploymentResponse struct {
	ID    uuid.UUID                   `json:"id"`
	State model.DeploymentStatusState `json:"state"`
}

type DeploymentStatus struct {
	State        model.DeploymentStatusState `json:"state"`
	Message      string                      `json:"message"`
	LastModified time.Time                   `json:"lastModified"`
	Created      time.Time                   `json:"created"`

	// DeploymentID  uuid.UUID `json:"-"`
	// EnvironmentID uuid.UUID `json:"-"`
}

type HttpHandler struct {
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	log      logrus.FieldLogger
	AllowAll bool

	programContext context.Context
	querier        restapisql.Querier
}

type Claims struct {
	Repository string `json:"repository"`
	Owner      string `json:"repository_owner"`
	Actor      string `json:"actor"`
	RunID      string `json:"run_id"`
}

type CreateDeploymentRequest struct {
	Chart       string             `json:"chart"`
	Version     string             `json:"version"`
	Description string             `json:"description"`
	Ref         *model.GHRef       `json:"ref"`
	Global      bool               `json:"global"`
	Target      environment.Labels `json:"target"`
}
