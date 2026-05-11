package deploymenttest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	Defaults         map[string]any
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

// AddDeploymentWithValues registers a deployment with configurable values and
// optional fake chart defaults. The defaults are returned by the seeder's
// ChartDownloader as the feature's ValuesYAML, mimicking values pulled from a
// real chart's values.yaml in production.
func (s *Seeder) AddDeploymentWithValues(name, version string, target environment.Labels, kinds []model.EnvironmentKind, values model.Values, defaults map[string]any, deps ...string) *Seeder {
	s.deployments = append(s.deployments, deploymentInput{
		FeatureName:      name,
		Version:          version,
		Target:           target,
		Dependencies:     deps,
		EnvironmentKinds: kinds,
		Values:           values,
		Defaults:         defaults,
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
			Ref:         fakeGHRef(d.FeatureName, d.Version),
			Target:      d.Target,
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
				valuesYAML, err := buildValuesYAML(deploy.Values, deploy.Defaults)
				if err != nil {
					return nil, fmt.Errorf("build defaults for %s: %w", deploy.FeatureName, err)
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
					ValuesYAML: valuesYAML,
					Source:     "https://example.com/" + deploy.FeatureName,
				}, nil
			}
		}
		return nil, fmt.Errorf("chartUrl %s with version %s not found in deployments", chartURL, version)
	}
}

// fakeGHRef produces a deterministic, realistic-looking commit SHA for seed
// deployments so the UI can render a sensible GitHub link in local dev.
func fakeGHRef(name, version string) *model.GHRef {
	sum := sha256.Sum256([]byte(name + "@" + version))
	return &model.GHRef{
		Owner: "nais",
		Repo:  name,
		Ref:   hex.EncodeToString(sum[:20]),
	}
}

func buildValuesYAML(values model.Values, defaults map[string]any) (map[string]json.RawMessage, error) {
	if len(defaults) == 0 {
		return nil, nil
	}
	out := map[string]json.RawMessage{}
	for k, v := range values {
		if v.Config == nil {
			continue
		}
		dv, ok := defaults[k]
		if !ok {
			continue
		}
		b, err := json.Marshal(dv)
		if err != nil {
			return nil, fmt.Errorf("marshal default for %q: %w", k, err)
		}
		out[k] = b
	}
	return out, nil
}
