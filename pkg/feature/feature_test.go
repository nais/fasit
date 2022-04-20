package feature

import (
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"
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
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			f := Feature{
				Config: test.Config,
			}
			rf := f.RequiredFields()

			if !cmp.Equal(rf, test.expected) {
				t.Error(cmp.Diff(rf, test.expected))
			}
		})
	}
}

func TestManager_ValidConfig(t *testing.T) {
	mgr := Manager{
		Features: []Feature{
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
		Features: []Feature{
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
		Features: []Feature{
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
		},
		{
			Name:    "nais-crds",
			Chart:   "oci://europe-north1-docker.pkg.dev/nais-io/nais/nais-crds",
			Version: "0.1.0",
			Source:  "https://github.com/nais/liberator/tree/main/charts",
			Config:  Config{},
		},
	}

	mgr, err := New(featureFS)
	if err != nil {
		t.Fatal(err)
	}

	if !cmp.Equal(mgr.Features, expected) {
		t.Error(cmp.Diff(mgr.Features, expected))
	}
}

var featureFS = fstest.MapFS{
	"features/cert-manager.yaml": &fstest.MapFile{
		Data: []byte(`
chart: cert-manager
source: https://github.com/cert-manager/cert-manager
version: v1.7.2
repo: https://charts.jetstack.io
config:
  installCRDs:
    type: bool
  global.podSecurityPolicy.enabled:
    type: bool`),
	},
	"features/nais-crds.yaml": &fstest.MapFile{
		Data: []byte(`
chart: oci://europe-north1-docker.pkg.dev/nais-io/nais/nais-crds
source: https://github.com/nais/liberator/tree/main/charts
version: 0.1.0
config: {}`),
	},
}
