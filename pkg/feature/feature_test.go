package feature

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/sirupsen/logrus"
)

func TestFeature_RequiredFields(t *testing.T) {
	tests := map[string]struct {
		Config
		expected []string
	}{
		"none": {
			Config: Config{
				"foo": ConfigType{Required: false},
				"bar": ConfigType{Required: false},
			},
			expected: nil,
		},
		"one": {
			Config: Config{
				"foo": ConfigType{Required: true},
				"bar": ConfigType{Required: false},
			},
			expected: []string{"foo"},
		},
		"all": {
			Config: Config{
				"foo": ConfigType{Required: true},
				"bar": ConfigType{Required: true},
			},
			expected: []string{"foo", "bar"},
		},
		"ignore management": {
			Config: Config{
				"foo": ConfigType{Required: true},
				"bar": ConfigType{Required: true, IgnoreKind: []model.EnvironmentKind{model.EnvironmentKindManagement}},
			},
			expected: []string{"foo"},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			f := Feature{
				Config: test.Config,
			}
			got := f.RequiredFields(model.EnvironmentKindManagement)

			opts := cmpopts.SortSlices(func(a, b string) bool {
				return strings.Compare(a, b) < 0
			})
			if !cmp.Equal(test.expected, got, opts) {
				t.Errorf("diff -want +got:\n%v", cmp.Diff(test.expected, got, opts))
			}
		})
	}
}

func TestNew(t *testing.T) {
	expected := []*model.Feature{
		{
			Name:    "cert-manager",
			Chart:   "cert-manager",
			Version: "v1.7.2",
			Source:  "https://github.com/cert-manager/cert-manager",
			FeatureYAML: model.FeatureYAML{Timeout: 15 * time.Minute, Values: model.Values{
				"global.podSecurityPolicy.enabled": model.Value{
					Config: &model.Config{
						Type: "bool",
					},
				},
				"installCRDs": {Config: &model.Config{Type: "bool"}},
			}},
		},
		{
			Name:        "nais-crds",
			Chart:       "oci://europe-north1-docker.pkg.dev/nais-io/nais/nais-crds",
			Version:     "0.1.0",
			Source:      "https://github.com/nais/liberator/tree/main/charts",
			FeatureYAML: model.FeatureYAML{Timeout: 5 * time.Minute, Values: model.Values{}},
		},
	}

	source, err := NewFeatureSourceFilesystem("./testdata", logrus.StandardLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	mgr, err := New(source, logrus.StandardLogger())
	if err != nil {
		t.Fatal(err)
	}

	if !cmp.Equal(mgr.Features(), expected) {
		t.Error(cmp.Diff(mgr.Features(), expected))
	}
}
