package graph

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v5"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/graphgen"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/naisdstatus"
)

// FeatureStates is the resolver for the featureStates field.
func (r *environmentResolver) FeatureStates(ctx context.Context, obj *model.Environment) ([]*model.FeatureState, error) {
	ret, err := deployment.ListEnvironmentFeatures(ctx, obj.ID)
	if err != nil {
		return nil, err
	}

	features := make(map[string]bool)
	for _, f := range ret {
		features[f.FeatureName] = true
	}

	states, err := featurepkg.FeatureStatesGet(ctx, obj.ID)
	if err != nil {
		return nil, err
	}

	for _, state := range states {
		if _, ok := features[state.FeatureName]; !ok {
			ret = append(ret, state)
		}
	}
	slices.SortFunc(ret, func(a, b *model.FeatureState) int {
		return strings.Compare(a.FeatureName, b.FeatureName)
	})

	return ret, nil
}

// GCPProjectID is the resolver for the gcpProjectID field.
func (r *environmentResolver) GCPProjectID(ctx context.Context, obj *model.Environment) (*string, error) {
	ev, err := r.Repo.EnvironmentValueGet(ctx, obj.ID, "project_id", false)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	if ev == nil {
		return nil, nil
	}

	var id string
	if err := json.Unmarshal(ev.Value, &id); err != nil {
		return nil, err
	}

	if id == "" {
		return nil, nil
	}

	return &id, nil
}

// Health is the resolver for the health field.
func (r *environmentResolver) Health(ctx context.Context, obj *model.Environment) (*model.Health, error) {
	health, err := naisdstatus.Get(ctx, obj.ID)
	if err != nil {
		return &model.Health{
			EnvironmentID: obj.ID,
			ReportedAt:    time.Date(1969, 6, 9, 6, 9, 6, 9, time.UTC),
		}, nil
	}
	return health, nil
}

// Releases is the resolver for the releases field.
func (r *environmentResolver) Releases(ctx context.Context, obj *model.Environment) ([]*model.Release, error) {
	return r.Repo.ReleaseStatusesGet(ctx, obj.ID)
}

// Nodes is the resolver for the nodes field.
func (r *environmentResolver) Nodes(ctx context.Context, obj *model.Environment) ([]*model.KubernetesNode, error) {
	return r.Repo.KubernetesNodesForEnv(ctx, obj.ID)
}

// Values is the resolver for the values field.
func (r *environmentResolver) Values(ctx context.Context, obj *model.Environment) ([]*model.EnvironmentValue, error) {
	return r.Repo.EnvironmentValuesForEnvironment(ctx, obj.ID, false)
}

// Tenant is the resolver for the tenant field.
func (r *environmentResolver) Tenant(ctx context.Context, obj *model.Environment) (*model.Tenant, error) {
	return environment.GetTenant(ctx, obj.TenantID)
}

// Warnings is the resolver for the warnings field.
func (r *environmentResolver) Warnings(ctx context.Context, obj *model.Environment) ([]model.Warning, error) {
	return environment.Warnings(ctx, &obj.ID, nil)
}

// AuditLog is the resolver for the auditLog field.
func (r *environmentResolver) AuditLog(ctx context.Context, obj *model.Environment, featureName *string) ([]*model.AuditLog, error) {
	fn := ""
	if featureName != nil {
		fn = *featureName
	}

	return audit.AuditForEnvironment(ctx, obj.ID, fn)
}

// Features is the resolver for the features field.
func (r *environmentResolver) Features(ctx context.Context, obj *model.Environment) ([]*model.Feature, error) {
	fs, err := featurepkg.FeaturesForKind(ctx, obj.Kind, obj.CI)
	if err != nil {
		return nil, err
	}

	for _, f := range fs {
		f.GraphVars.EnvironmentID = obj.ID
	}

	return fs, nil
}

// Feature is the resolver for the feature field.
func (r *environmentResolver) Feature(ctx context.Context, obj *model.Environment, name string) (*model.Feature, error) {
	f, err := featurepkg.FeatureByNameForEnv(ctx, name, obj.ID)
	if err != nil {
		return nil, err
	}

	// copy
	b, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}

	ret := &model.Feature{}
	if err := json.Unmarshal(b, ret); err != nil {
		return nil, err
	}

	ret.GraphVars.EnvironmentID = obj.ID

	return ret, nil
}

// Labels is the resolver for the labels field.
func (r *environmentResolver) Labels(ctx context.Context, obj *model.Environment) ([]*model.EnvironmentLabel, error) {
	return r.Repo.EnvironmentGetLabels(ctx, obj.ID)
}

// EnvironmentCreate is the resolver for the environmentCreate field.
func (r *mutationResolver) EnvironmentCreate(ctx context.Context, environment model.EnvironmentCreate) (*model.Environment, error) {
	return r.Repo.EnvironmentCreate(ctx, &environment)
}

// EnvironmentUpdate is the resolver for the environmentUpdate field.
func (r *mutationResolver) EnvironmentUpdate(ctx context.Context, id uuid.UUID, input model.EnvironmentUpdate) (*model.Environment, error) {
	return r.Repo.EnvironmentUpdate(ctx, id, &input)
}

// EnvironmentSetReconcile is the resolver for the environmentSetReconcile field.
func (r *mutationResolver) EnvironmentSetReconcile(ctx context.Context, id uuid.UUID, reconcile bool) (*model.Environment, error) {
	return r.Repo.EnvironmentSetReconcile(ctx, id, reconcile)
}

// Feature is the resolver for the feature field.
func (r *releaseResolver) Feature(ctx context.Context, obj *model.Release) (*model.Feature, error) {
	f, err := featurepkg.FeatureByNameForEnv(ctx, obj.Name, obj.GraphVars.EnvironmentID)
	if err != nil {
		r.Log.WithError(err).Debug("error getting feature for release, returning nil")
	}

	if f == nil {
		return nil, nil
	}

	f.GraphVars.EnvironmentID = obj.GraphVars.EnvironmentID

	return f, nil
}

func (r *Resolver) Environment() graphgen.EnvironmentResolver { return &environmentResolver{r} }

func (r *Resolver) Release() graphgen.ReleaseResolver { return &releaseResolver{r} }

type (
	environmentResolver struct{ *Resolver }
	releaseResolver     struct{ *Resolver }
)
