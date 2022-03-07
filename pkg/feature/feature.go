package feature

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

type ConfigType struct {
	Type   string
	Secret bool
}

func (c ConfigType) Valid(value json.RawMessage) bool {
	if c.Type == "" {
		return false
	}

	var v any
	if err := json.Unmarshal(value, &v); err != nil {
		return false
	}

	switch v.(type) {
	case string:
		return c.Type == "string"
	case int:
		return c.Type == "int"
	case bool:
		return c.Type == "bool"
	case []string:
		return c.Type == "stringarray"
	default:
		return false
	}
}

func (c *ConfigType) UnmarshalYAML(unmarshal func(any) error) error {
	var v string
	if err := unmarshal(&v); err != nil {
		return err
	}
	parts := strings.SplitN(v, ",", 2)
	switch parts[0] {
	case "string", "int", "bool", "stringarray":
		c.Type = parts[0]
	default:
		return fmt.Errorf("unsupported config type %q", parts[0])
	}

	if len(parts) == 2 {
		if parts[1] == "secret" {
			c.Secret = true
		} else {
			return fmt.Errorf("unsupported config option %q", parts[1])
		}
	}
	return nil
}

type Config map[string]ConfigType

type Feature struct {
	Name    string `yaml:"name"`
	Chart   string `yaml:"chart"`
	Version string `yaml:"version"`
	Repo    string `yaml:"repo"`
	Source  string `yaml:"source"`
	Config  Config `yaml:"config"`
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

func (m *Manager) ValidConfig(feature, key string, value json.RawMessage) bool {
	for _, f := range m.Features {
		if f.Name == feature {
			return f.Config[key].Valid(value)
		}
	}
	return false
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

// func (m *Manager) EnsureKeysInDB(ctx context.Context, repo *database.Repo) error {
// 	// Iterate over all features
// 	// If feature is not in DB, add it
// }
