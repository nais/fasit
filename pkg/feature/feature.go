package feature

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/helm"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
)

type Values map[string]Value

type Computed struct {
	// Template is a Go template which must return a valid YAML element.
	Template string `yaml:"template,omitempty" json:"template,omitempty"`
}

type Config struct {
	// Type is the type of the config.
	Type model.ConfigType `yaml:"type,omitempty" json:"type,omitempty"`
	// Secret is true if the config is a secret.
	Secret bool `json:"secret,omitempty" yaml:"secret,omitempty"`
}

type Value struct {
	// Description is a short description of the value.
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	// DisplayName is the name of the value that will be displayed to the user.
	DisplayName string `yaml:"displayName,omitempty" json:"displayName,omitempty"`
	// Required is true if the config is required.
	Required bool `yaml:"required,omitempty" json:"required,omitempty"`
	// Computed is a computed value that will be set if no config is set.
	Computed *Computed `yaml:"computed,omitempty" json:"computed,omitempty"`
	// Config specifies how the value should be configured.
	Config *Config `yaml:"config,omitempty" json:"config,omitempty"`
}

type FeatureYAML struct {
	// Dependencies defines the features that this feature depends on.
	Dependencies Dependencies `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	// EnvironmentKinds is the list of environments this feature can be used in.
	EnvironmentKinds []model.EnvironmentKind `yaml:"environmentKinds" json:"environmentKinds" jsonschema:"enum=management,enum=tenant,enum=onprem,enum=legacy,required"`
	// Timeout is the amount of time helm should wait for the feature to be ready. Defaults to 5m0s
	Timeout time.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty" jsonschema:"omitempty,type=string,pattern=^(\\d+h)?(\\d+m)?(\\d+s)?$"`
	// Values is a list of values that can be overridden by the user.
	Values Values `yaml:"values,omitempty" json:"values,omitempty"`
}

type Feature struct {
	FeatureYAML

	Name string `yaml:"name" json:"name" jsonschema:"-"`
	// Chart name if using helm charts or full url if using CRI image chart.
	Chart string `yaml:"chart" json:"chart"`
	// Version of the chart.
	Version string `yaml:"version" json:"version"`
	// Description is a short description of the feature.
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	// Source should be the URL to the helm chart source code.
	Source     string         `yaml:"source" json:"source"`
	ValuesYaml map[string]any `yaml:"-" json:"-"`
}

func FromChart(chart, version string) (*Feature, error) {
	resp, err := helm.DownloadChart(chart, version, "")
	if err != nil {
		return nil, err
	}

	gr, err := gzip.NewReader(resp)
	if err != nil {
		return nil, err
	}

	f := &Feature{
		Chart: chart,
	}

	r := tar.NewReader(gr)
	for {
		hdr, err := r.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		fname := strings.Split(hdr.Name, "/")
		// Ensure that the file is in the root of the chart
		if len(fname) != 2 {
			continue
		}

		switch fname[1] {
		case "Chart.yaml":
			if err := f.parseChartYAML(r); err != nil {
				return nil, err
			}
		case "Feature.yaml":
			if err := yaml.NewDecoder(r).Decode(&f.FeatureYAML); err != nil {
				return nil, err
			}
		case "values.yaml":
			b, err := io.ReadAll(r)
			if err != nil {
				return nil, err
			}
			vals, err := chartutil.ReadValues(b)
			if err != nil {
				return nil, err
			}
			f.ValuesYaml = vals
		}
	}

	return f, nil
}

func (f *Feature) parseChartYAML(r io.Reader) error {
	meta := &chart.Metadata{}
	if err := yaml.NewDecoder(r).Decode(meta); err != nil {
		return err
	}

	f.Name = meta.Name
	f.Version = meta.Version
	f.Description = meta.Description
	if len(meta.Sources) > 0 {
		f.Source = meta.Sources[0]
	}

	return nil
}

type Manager struct {
	lock     sync.RWMutex
	features []Feature
}

func New(log logrus.FieldLogger) (*Manager, error) {
	// mgr := &Manager{}
	// features, err := source.Features()
	// if err != nil {
	// 	return nil, err
	// }

	// source.Register(func() {
	// 	features, err := source.Features()
	// 	if err != nil {
	// 		log.WithError(err).Error("failed to reload features")
	// 		return
	// 	}
	// 	mgr.SetFeatures(features)
	// })

	// mgr.SetFeatures(features)
	// return mgr, nil
	panic("not implemented")
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
	// return f.Config[key].Valid(value)
	return nil
}

func (m *Manager) IsSecret(feature, key string) bool {
	// for _, f := range m.Features() {
	// 	if f.Name == feature {
	// 		if c, ok := f.Config[key]; ok {
	// 			return c.Secret
	// 		}
	// 		break
	// 	}
	// }
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
		Name: strings.TrimSuffix(filepath.Base(filename), ".yaml"),
		FeatureYAML: FeatureYAML{
			Timeout: 5 * time.Minute,
		},
	}

	if err := yaml.NewDecoder(r).Decode(&feature); err != nil {
		return Feature{}, err
	}
	return feature, nil
}
