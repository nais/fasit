package deploymenttest

import (
	"context"
	"fmt"

	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/graph/model"
)

type Seeder struct {
	deployments deployments
}

type deploymentInput struct {
	FeatureName  string
	Version      string
	Target       environment.Labels
	Dependencies []string
}

type deployments []deploymentInput

func NewSeeder() *Seeder {
	return &Seeder{
		deployments: deployments{},
	}
}

func (s *Seeder) AddDeployment(name, version string, target environment.Labels, deps ...string) *Seeder {
	s.deployments = append(s.deployments, deploymentInput{
		FeatureName:  name,
		Version:      version,
		Target:       target,
		Dependencies: deps,
	})
	return s
}

func (s *Seeder) Seed(ctx context.Context, dmgr *deployment.Manager) error {
	for _, d := range s.deployments {
		_, err := dmgr.CreateDeployment(ctx, deployment.Request{
			Chart:       "oci://" + d.FeatureName,
			Version:     d.Version,
			Description: "Setup local environment deployment",
			Global:      true,
			Ref: &model.GHRef{
				Owner: "nais",
				Repo:  "fasit",
				Ref:   "refs/heads/main",
			},
			Target: d.Target,
			SkipCI: true,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Seeder) Reset() {
	s.deployments = deployments{}
}

func (s *Seeder) ChartDownloader() deployment.ChartDownloader {
	return func(chartURL, version string) (*model.Feature, error) {
		for _, deploy := range s.deployments {
			u := "oci://" + deploy.FeatureName
			if u == chartURL && deploy.Version == version {
				var deps model.Dependencies
				if len(deploy.Dependencies) > 0 {
					deps = model.Dependencies{
						&model.Dependency{
							AllOf: deploy.Dependencies,
						},
					}
				}
				return &model.Feature{
					Name:    deploy.FeatureName,
					Version: deploy.Version,
					Chart:   u,
					FeatureYAML: model.FeatureYAML{
						Dependencies:     deps,
						EnvironmentKinds: []model.EnvironmentKind{"tenant", "management"},
					},
					Source: "https://example.com/" + deploy.FeatureName,
				}, nil
			}
		}
		return nil, fmt.Errorf("chartUrl %s with version %s not found in deployments", chartURL, version)
	}
}
