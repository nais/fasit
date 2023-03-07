package workers

import (
	"context"
	"fmt"
	"time"

	"github.com/nais/fasit/pkg/database"
	feature "github.com/nais/fasit/pkg/feature2"
	"github.com/nais/fasit/pkg/graph/model"
)

type AutoInstaller struct {
	repo database.Repo
}

func NewAutoInstaller(repo database.Repo) *AutoInstaller {
	return &AutoInstaller{
		repo: repo,
	}
}

func (a *AutoInstaller) Run(ctx context.Context, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := a.ensure(ctx); err != nil {
				return err
			}
		}
	}
}

func (a *AutoInstaller) ensure(ctx context.Context) error {
	envs, err := a.repo.TenantEnvironments(ctx)
	if err != nil {
		return fmt.Errorf("get all tenants environments: %w", err)
	}

	for _, env := range envs {
		if err := a.ensureEnvironment(ctx, env); err != nil {
			return fmt.Errorf("ensure environment %s: %w", env.Name, err)
		}
	}

	return nil
}

func (a *AutoInstaller) ensureEnvironment(ctx context.Context, env *model.TenantEnvironment) error {
	desired, err := a.desiredFeaturesForEnv(ctx, env)
	if err != nil {
		return err
	}

	states, err := a.repo.FeatureStatesGet(ctx, env.ID)
	if err != nil {
		return fmt.Errorf("get feature states for environment %s: %w", env.Name, err)
	}

	isEnabled := func(featureName string, states []*model.FeatureState) bool {
		for _, state := range states {
			if state.FeatureName == featureName {
				return state.Enabled
			}
		}
		return false
	}

	for _, f := range desired {
		if !isEnabled(f.Name, states) {
			_, err := a.repo.FeatureStatesCreateOrUpdate(ctx, env.ID, f, true)
			if err != nil {
				return fmt.Errorf("enable feature %s for environment %s: %w", f.Name, env.Name, err)
			}
		}
	}

	return nil
}

func (a *AutoInstaller) desiredFeaturesForEnv(ctx context.Context, env *model.TenantEnvironment) ([]*feature.Feature, error) {
	features, err := a.repo.FeaturesForKind(ctx, env.Kind, env.CI)
	if err != nil {
		return nil, fmt.Errorf("get features for kind %s: %w", env.Kind, err)
	}

	autoInstalls, err := a.repo.AutoInstallsForKind(ctx, env.Kind)
	if err != nil {
		return nil, fmt.Errorf("get auto installs for kind %s: %w", env.Kind, err)
	}

	ret := []*feature.Feature{}

	for _, ai := range autoInstalls {
		for _, feature := range features {
			if feature.Name == ai {
				ret = append(ret, feature)
			}
		}
	}
	return ret, nil
}
