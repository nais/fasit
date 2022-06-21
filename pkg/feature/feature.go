package feature

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/nais/fasit/pkg/graph/model"
	"gopkg.in/yaml.v2"
)

type Config map[string]ConfigType

type Feature struct {
	Name string `yaml:"name" json:"-" jsonschema:"-"`
	// Chart name if using helm charts or full url if using CRI image chart.
	Chart string `yaml:"chart"`
	// Version of the chart.
	Version string `yaml:"version"`
	// Repo is the repository where the helm chart is located.
	Repo string `yaml:"repo,omitempty"`
	// Source should be the URL to the helm chart source code.
	Source string `yaml:"source"`
	// DependsOn defines the features that this feature depends on.
	DependsOn []string `yaml:"dependsOn,omitempty"`
	// Config is the list of configuration options for the feature.
	Config Config `yaml:"config,omitempty"`
	// Mapping is the list of mappings from environment values for the feature.
	Mapping Mapping `yaml:"mapping,omitempty"`
	// EnvironmentKinds is the list of environments this feature can be used in.
	EnvironmentKinds []model.EnvironmentKind `yaml:"environmentKinds" jsonschema:"enum=management,enum=tenant,required"`
	// Timeout is the amount of time helm should wait for the feature to be ready. Defaults to 5m0s
	Timeout time.Duration `yaml:"timeout,omitempty" jsonschema:"omitempty,type=string,pattern=^(\\d+h)?(\\d+m)?(\\d+s)?$"`
}

func (f *Feature) RequiredFields() []string {
	var requiredFields []string
	for k, v := range f.Config {
		if v.Required {
			requiredFields = append(requiredFields, k)
		}
	}
	return requiredFields
}

type Manager struct {
	Features []Feature
}

func New(files fs.FS) (*Manager, error) {
	features := []Feature{}
	err := fs.WalkDir(files, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		f, err := fs.ReadFile(files, path)
		if err != nil {
			return err
		}

		feature := Feature{
			Name:    strings.TrimSuffix(filepath.Base(path), ".yaml"),
			Timeout: 5 * time.Minute,
		}
		err = yaml.Unmarshal(f, &feature)
		if err != nil {
			return err
		}

		features = append(features, feature)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &Manager{
		Features: features,
	}, nil
}

func (m *Manager) ValidConfig(feature, key string, value json.RawMessage) error {
	f := m.Get(feature)
	if f == nil {
		return fmt.Errorf("%q not a valid feature", feature)
	}
	return f.Config[key].Valid(value)
}

func (m *Manager) IsSecret(feature, key string) bool {
	for _, f := range m.Features {
		if f.Name == feature {
			if c, ok := f.Config[key]; ok {
				return c.Secret
			}
			break
		}
	}
	return false
}

func (m *Manager) Get(feature string) *Feature {
	for _, f := range m.Features {
		if f.Name == feature {
			return &f
		}
	}
	return nil
}
