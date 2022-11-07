package feature

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/nais/fasit/pkg/graph/model"
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

func TestManager_ValidConfig(t *testing.T) {
	mgr := Manager{
		features: []Feature{
			{
				Name: "foo",
				Config: Config{
					"key": ConfigType{
						Type: model.ConfigTypeString,
					},
				},
			},
		},
	}

	if err := mgr.ValidConfig("foo", "key", []byte(`"value"`)); err != nil {
		t.Error(err)
	}
	if err := mgr.ValidConfig("foo", "key", []byte(`123`)); err == nil {
		t.Error("expected error, but got none")
	}
	if err := mgr.ValidConfig("bar", "key", []byte(`123`)); err == nil {
		t.Error("expected error, but got none")
	}
}

func TestManager_IsSecret(t *testing.T) {
	mgr := Manager{
		features: []Feature{
			{
				Name: "foo",
				Config: Config{
					"sensitive": ConfigType{
						Secret: true,
					},
					"public": ConfigType{
						Secret: false,
					},
				},
			},
		},
	}

	if !mgr.IsSecret("foo", "sensitive") {
		t.Error("expected secret to be true")
	}

	if mgr.IsSecret("foo", "public") {
		t.Error("expected secret to be false")
	}

	if mgr.IsSecret("foo", "nonexisting") {
		t.Error("expected secret to be false")
	}
}

func TestManager_Get(t *testing.T) {
	mgr := Manager{
		features: []Feature{
			{Name: "foo"},
		},
	}

	f := mgr.Get("foo")
	if f == nil {
		t.Error("expected feature to be found")
	}

	f = mgr.Get("bar")
	if f != nil {
		t.Error("expected feature to not be found")
	}
}

func TestNew(t *testing.T) {
	expected := []Feature{
		{
			Name:    "cert-manager",
			Chart:   "cert-manager",
			Version: "v1.7.2",
			Repo:    "https://charts.jetstack.io",
			Source:  "https://github.com/cert-manager/cert-manager",
			Config: Config{
				"global.podSecurityPolicy.enabled": {Type: model.ConfigTypeBool},
				"installCRDs":                      {Type: model.ConfigTypeBool},
			},
			Timeout: 15 * time.Minute,
		},
		{
			Name:    "nais-crds",
			Chart:   "oci://europe-north1-docker.pkg.dev/nais-io/nais/nais-crds",
			Version: "0.1.0",
			Source:  "https://github.com/nais/liberator/tree/main/charts",
			Config:  Config{},
			Timeout: 5 * time.Minute, // default
		},
	}

	source, err := NewFeatureSourceFilesystem("./testdata")
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	mgr, err := New(source)
	if err != nil {
		t.Fatal(err)
	}

	if !cmp.Equal(mgr.Features(), expected) {
		t.Error(cmp.Diff(mgr.Features(), expected))
	}
}
