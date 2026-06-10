package api

import (
	"context"
	"log/slog"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/api/sqlgen"
	"github.com/nais/fasit/internal/database/types"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/model"
)

type GetFeatureAssignmentResponse struct {
	ID    uuid.UUID                         `json:"id"`
	State model.FeatureReconcileStatusState `json:"state"`
}

type FeatureReconcileStatus struct {
	State        model.FeatureReconcileStatusState `json:"state"`
	Message      string                            `json:"message"`
	LastModified time.Time                         `json:"lastModified"`
	Created      time.Time                         `json:"created"`

	// FeatureAssignmentID  uuid.UUID `json:"-"`
	// EnvironmentID uuid.UUID `json:"-"`
}

type HttpHandler struct {
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	log      *slog.Logger
	AllowAll bool

	programContext context.Context
	querier        sqlgen.Querier
}

type Claims struct {
	Repository string `json:"repository"`
	Owner      string `json:"repository_owner"`
	Actor      string `json:"actor"`
	RunID      string `json:"run_id"`
}

type CreateFeatureAssignmentRequest struct {
	Chart       string              `json:"chart"`
	Version     string              `json:"version"`
	Description string              `json:"description"`
	Ref         *model.GitHubCommit `json:"ref"`
	Global      bool                `json:"global"`
	Target      environment.Labels  `json:"target"`
}

type Tenant struct {
	ID           uuid.UUID     `json:"id"`
	Name         string        `json:"name"`
	Environments []Environment `json:"environments"`
}
type Environment struct {
	ID     uuid.UUID               `json:"id"`
	Name   string                  `json:"name"`
	Kind   types.EnvironmentKind   `json:"kind"`
	Labels types.EnvironmentLabels `json:"labels"`
}
