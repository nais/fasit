package feature

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

type Config map[string]ConfigType

type Feature struct {
	Name    string `yaml:"name"`
	Chart   string `yaml:"chart"`
	Version string `yaml:"version"`
	Repo    string `yaml:"repo"`
	Source  string `yaml:"source"`
	Config  Config `yaml:"config"`
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
			Name: strings.TrimSuffix(filepath.Base(path), ".yaml"),
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
