package feature

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/nais/fasit/pkg/graph/model"
	"gopkg.in/yaml.v2"
)

type ConfigType struct {
	Type   model.ConfigType `json:"type"`
	Secret bool             `json:"secret"`
}

func (c ConfigType) Valid(value json.RawMessage) error {
	if c.Type == "" {
		return fmt.Errorf("type is invalid")
	}

	var v any
	if err := json.Unmarshal(value, &v); err != nil {
		return fmt.Errorf("unable to decode json: %w", err)
	}

	switch v.(type) {
	case string:
		if c.Type == model.ConfigTypeString {
			return nil
		}
		return fmt.Errorf("value doesn't match the required type. Expected string, got %T", v)
	case int:
		if c.Type == model.ConfigTypeInt {
			return nil
		}
		return fmt.Errorf("value doesn't match the required type. Expected int, got %T", v)
	case bool:
		if c.Type == model.ConfigTypeBool {
			return nil
		}
		return fmt.Errorf("value doesn't match the required type. Expected bool, got %T", v)
	case []string:
		if c.Type == model.ConfigTypeStringArray {
			return nil
		}
		return fmt.Errorf("value doesn't match the required type. Expected []string, got %T", v)
	default:
		return nil
	}
}

func (c *ConfigType) UnmarshalYAML(unmarshal func(any) error) error {
	var v string
	if err := unmarshal(&v); err != nil {
		return err
	}
	parts := strings.SplitN(v, ",", 2)
	c.Type = model.ConfigType(parts[0])
	if !c.Type.IsValid() {
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

// func (m *Manager) EnsureKeysInDB(ctx context.Context, repo *database.Repo) error {
// 	// Iterate over all features
// 	// If feature is not in DB, add it
// }
