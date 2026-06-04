package featureassignmenttest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/graph/model"
	commonmodel "github.com/nais/fasit/internal/model"
)

type Seeder struct {
	assignments assignments
}

type assignmentInput struct {
	FeatureName      string
	Version          string
	Target           environment.Labels
	Dependencies     []string
	EnvironmentKinds []model.EnvironmentKind
	Values           model.Values
	Defaults         map[string]any
	Description      string
}

type assignments []assignmentInput

func (d assignmentInput) kinds() []model.EnvironmentKind {
	if len(d.EnvironmentKinds) > 0 {
		return d.EnvironmentKinds
	}
	return []model.EnvironmentKind{"tenant", "management"}
}

func NewSeeder() *Seeder {
	return &Seeder{}
}

func (s *Seeder) AddAssignment(name, version string, target environment.Labels, deps ...string) *Seeder {
	s.assignments = append(s.assignments, assignmentInput{
		FeatureName:  name,
		Version:      version,
		Target:       target,
		Dependencies: deps,
	})
	return s
}

// AddAssignmentWithValues registers a feature assignment with configurable values and
// optional fake chart defaults. The defaults are returned by the seeder's
// ChartDownloader as the feature's ValuesYAML, mimicking values pulled from a
// real chart's values.yaml in production.
func (s *Seeder) AddAssignmentWithValues(name, version string, target environment.Labels, kinds []model.EnvironmentKind, values model.Values, defaults map[string]any, description string, deps ...string) *Seeder {
	s.assignments = append(s.assignments, assignmentInput{
		FeatureName:      name,
		Version:          version,
		Target:           target,
		Dependencies:     deps,
		EnvironmentKinds: kinds,
		Values:           values,
		Defaults:         defaults,
		Description:      description,
	})
	return s
}

func (s *Seeder) Seed(ctx context.Context) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(s.assignments))
	for _, d := range s.assignments {
		id, err := s.create(ctx, d)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// CreateAssignment registers the fake chart and creates the assignment in one
// step, returning its id. Unlike Add + Seed, it persists immediately, so tests
// can read as a linear sequence of actions.
func (s *Seeder) CreateAssignment(ctx context.Context, name, version string, target environment.Labels, deps ...string) (uuid.UUID, error) {
	s.AddAssignment(name, version, target, deps...)
	return s.create(ctx, s.assignments[len(s.assignments)-1])
}

// CreateAssignmentWithValues is CreateAssignment with configurable values and
// fake chart defaults; see AddAssignmentWithValues.
func (s *Seeder) CreateAssignmentWithValues(ctx context.Context, name, version string, target environment.Labels, kinds []model.EnvironmentKind, values model.Values, defaults map[string]any, description string, deps ...string) (uuid.UUID, error) {
	s.AddAssignmentWithValues(name, version, target, kinds, values, defaults, description, deps...)
	return s.create(ctx, s.assignments[len(s.assignments)-1])
}

func (s *Seeder) create(ctx context.Context, d assignmentInput) (uuid.UUID, error) {
	return featureassignment.Create(ctx, featureassignment.CreateFeatureAssignment{
		Chart:       "oci://" + d.FeatureName,
		Version:     d.Version,
		Description: new("Setup local environment assignment"),
		Commit:      fakeGitHubCommit(d.FeatureName, d.Version),
		Target:      d.Target,
	})
}

func (s *Seeder) Reset() {
	s.assignments = nil
}

func (s *Seeder) ChartDownloader() featureassignment.ChartDownloaderFunc {
	return func(chartURL, version string) (*model.Feature, error) {
		for _, deploy := range s.assignments {
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
					Name:        deploy.FeatureName,
					Version:     deploy.Version,
					Chart:       u,
					Description: deploy.Description,
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
		return nil, fmt.Errorf("chartUrl %s with version %s not found in feature assignments", chartURL, version)
	}
}

// fakeGitHubCommit produces a deterministic, realistic-looking commit SHA for seed
// deployments so the UI can render a sensible GitHub link in local dev.
func fakeGitHubCommit(name, version string) *commonmodel.GitHubCommit {
	sum := sha256.Sum256([]byte(name + "@" + version))
	return &commonmodel.GitHubCommit{
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
