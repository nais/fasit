package graph

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-jose/go-jose/v4/json"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/graph/graphgen"
	"github.com/nais/fasit/internal/graph/model"
)

// Statuses is the resolver for the deploymentStatuses field.
func (r *deploymentResolver) Statuses(ctx context.Context, obj *deployment.Deployment) ([]*deployment.DeploymentStatus, error) {
	return deployment.ListDeploymentStatuses(ctx, obj.ID)
}

// Deployment is the resolver for the deployment field.
func (r *deploymentStatusResolver) Deployment(ctx context.Context, obj *deployment.DeploymentStatus) (*deployment.Deployment, error) {
	return deployment.GetDeployment(ctx, obj.DeploymentID)
}

// Environment is the resolver for the environment field.
func (r *deploymentStatusResolver) Environment(ctx context.Context, obj *deployment.DeploymentStatus) (*model.Environment, error) {
	return r.Repo.EnvironmentGet(ctx, obj.EnvironmentID)
}

// CreateDeployment is the resolver for the createDeployment field.
func (r *mutationResolver) CreateDeployment(ctx context.Context, input model.CreateDeploymentInput) (uuid.UUID, error) {
	if input.Target == "" {
		return uuid.Nil, fmt.Errorf("target must be a valid JSON string")
	}

	target := make(environment.Labels)
	if err := json.NewDecoder(strings.NewReader(input.Target)).Decode(&target); err != nil {
		return uuid.Nil, fmt.Errorf("invalid target: %w", err)
	}

	deploymentID, err := deployment.CreateDeployment(ctx, deployment.Request{
		Chart:       input.Chart,
		Version:     input.Version,
		Description: input.Description,
		Global:      input.Global,
		Target:      target,
		CI: struct {
			Skip bool `json:"skip"`
			Wait bool `json:"wait"`
		}{
			Skip: true,
			Wait: true,
		},
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("create deployment: %w", err)
	}

	deployment.TriggerReconcile(ctx, deployment.ReconcileTriggerEvent{})

	return deploymentID, nil
}

// DeleteDeployment is the resolver for the deleteDeployment field.
func (r *mutationResolver) DeleteDeployment(ctx context.Context, deploymentID uuid.UUID) (bool, error) {
	if err := deployment.DeleteDeployment(ctx, deploymentID); err != nil {
		return false, fmt.Errorf("delete deployment: %w", err)
	}

	return true, nil
}

// Deployments is the resolver for the deployments field.
func (r *queryResolver) Deployments(ctx context.Context, feature *string) ([]*deployment.Deployment, error) {
	if feature != nil {
		return deployment.ListDeploymentsByFeature(ctx, *feature)
	}

	return deployment.ListDeployments(ctx)
}

// Deployment is the resolver for the deployment field.
func (r *queryResolver) Deployment(ctx context.Context, id uuid.UUID) (*deployment.Deployment, error) {
	return deployment.GetDeployment(ctx, id)
}

func (r *Resolver) Deployment() graphgen.DeploymentResolver { return &deploymentResolver{r} }

func (r *Resolver) DeploymentStatus() graphgen.DeploymentStatusResolver {
	return &deploymentStatusResolver{r}
}

type (
	deploymentResolver       struct{ *Resolver }
	deploymentStatusResolver struct{ *Resolver }
)
