package feature

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nais/fasit/pkg/graph/model"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type RolloutSource struct {
	Org  string `json:"org" yaml:"org"`
	Repo string `json:"repo" yaml:"repo"`
}

func (r RolloutSource) String() string {
	return r.Org + "/" + r.Repo
}

type Config map[string]ConfigType

type Feature struct {
	Name string `yaml:"name" json:"name" jsonschema:"-"`
	// Chart name if using helm charts or full url if using CRI image chart.
	Chart string `yaml:"chart" json:"chart"`
	// Version of the chart.
	Version string `yaml:"version" json:"version"`
	// Repo is the repository where the helm chart is located.
	Repo string `yaml:"repo,omitempty" json:"repo,omitempty"`
	// RolloutSource is the org and name of repositories which can trigger a rollout.
	RolloutSource []RolloutSource `yaml:"rolloutSource,omitempty" json:"rolloutSource,omitempty"`
	// Source should be the URL to the helm chart source code.
	Source string `yaml:"source" json:"source"`
	// DependsOn defines the features that this feature depends on.
	DependsOn Dependencies `yaml:"dependsOn,omitempty" json:"dependsOn,omitempty"`
	// Config is the list of configuration options for the feature.
	Config Config `yaml:"config,omitempty" json:"config,omitempty"`
	// Mapping is the list of mappings from environment values for the feature.
	Mapping Mapping `yaml:"mapping,omitempty" json:"mapping,omitempty"`
	// EnvironmentKinds is the list of environments this feature can be used in.
	EnvironmentKinds []model.EnvironmentKind `yaml:"environmentKinds" json:"environmentKinds" jsonschema:"enum=management,enum=tenant,enum=onprem,enum=legacy,required"`
	// AutoInstall is the list of environments this feature can be auto-installed in.
	AutoInstall []model.EnvironmentKind `yaml:"autoInstall,omitempty" json:"autoInstall,omitempty" jsonschema:"enum=management,enum=tenant,enum=onprem,enum=legacy"`
	// Timeout is the amount of time helm should wait for the feature to be ready. Defaults to 5m0s
	Timeout time.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty" jsonschema:"omitempty,type=string,pattern=^(\\d+h)?(\\d+m)?(\\d+s)?$"`
}

func (f *Feature) RequiredFields(envKind model.EnvironmentKind) []string {
	var requiredFields []string
	for k, v := range f.Config {
		if v.IgnoreKind.Contains(envKind) {
			continue
		}
		if v.Required {
			requiredFields = append(requiredFields, k)
		}
	}
	return requiredFields
}

type Manager struct {
	lock     sync.RWMutex
	features []Feature
}

func New(source FeatureSource, log logrus.FieldLogger) (*Manager, error) {
	mgr := &Manager{}
	features, err := source.Features()
	if err != nil {
		return nil, err
	}

	source.Register(func() {
		features, err := source.Features()
		if err != nil {
			log.WithError(err).Error("failed to reload features")
			return
		}
		mgr.SetFeatures(features)
	})

	mgr.SetFeatures(features)
	return mgr, nil
}

func (m *Manager) Features() []Feature {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.features[:]
}

func (m *Manager) SetFeatures(features []Feature) {
	// sorted because we want to be deterministic
	sort.Slice(features, func(i, j int) bool {
		return features[i].Name < features[j].Name
	})

	m.lock.Lock()
	defer m.lock.Unlock()
	m.features = features
}

func (m *Manager) ValidConfig(feature, key string, value json.RawMessage) error {
	f := m.Get(feature)
	if f == nil {
		return fmt.Errorf("%q not a valid feature", feature)
	}
	return f.Config[key].Valid(value)
}

func (m *Manager) IsSecret(feature, key string) bool {
	for _, f := range m.Features() {
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
	for _, f := range m.Features() {
		if f.Name == feature {
			return &f
		}
	}
	return nil
}

func parseFeature(filename string, r io.Reader) (Feature, error) {
	feature := Feature{
		Name:    strings.TrimSuffix(filepath.Base(filename), ".yaml"),
		Timeout: 5 * time.Minute,
	}

	if err := yaml.NewDecoder(r).Decode(&feature); err != nil {
		return Feature{}, err
	}
	return feature, nil
}
