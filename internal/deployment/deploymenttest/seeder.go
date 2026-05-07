package deploymenttest

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/graph/model"
)

type Seeder struct {
	deployments deployments
}

type deploymentInput struct {
	FeatureName      string
	Version          string
	Target           environment.Labels
	Dependencies     []string
	EnvironmentKinds []model.EnvironmentKind
	Values           model.Values
}

type deployments []deploymentInput

func (d deploymentInput) kinds() []model.EnvironmentKind {
	if len(d.EnvironmentKinds) > 0 {
		return d.EnvironmentKinds
	}
	return []model.EnvironmentKind{"tenant", "management"}
}

func NewSeeder() *Seeder {
	return &Seeder{}
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

func (s *Seeder) AddDeploymentWithValues(name, version string, target environment.Labels, kinds []model.EnvironmentKind, values model.Values, deps ...string) *Seeder {
	s.deployments = append(s.deployments, deploymentInput{
		FeatureName:      name,
		Version:          version,
		Target:           target,
		Dependencies:     deps,
		EnvironmentKinds: kinds,
		Values:           values,
	})
	return s
}

func (s *Seeder) Seed(ctx context.Context) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(s.deployments))
	for _, d := range s.deployments {
		id, err := deployment.CreateDeployment(ctx, deployment.Request{
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
		})
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *Seeder) Reset() {
	s.deployments = nil
}

func (s *Seeder) ChartDownloader() deployment.ChartDownloaderFunc {
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
						EnvironmentKinds: deploy.kinds(),
						Values:           deploy.Values,
					},
					Source: "https://example.com/" + deploy.FeatureName,
				}, nil
			}
		}
		return nil, fmt.Errorf("chartUrl %s with version %s not found in deployments", chartURL, version)
	}
}
