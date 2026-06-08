package feature

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/feature/featuresql"
)

func TestMergeConfigs_PreservesIDAndCreated(t *testing.T) {
	globalID := uuid.New()
	envID := uuid.New()
	environmentID := uuid.New()
	globalCreated := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	envCreated := time.Date(2025, 2, 2, 0, 0, 0, 0, time.UTC)

	globals := []featuresql.ConfigurationsGlobal{
		{ID: globalID, Feature: "f", Key: "only-global", Value: []byte(`"g"`), Created: globalCreated},
	}
	envs := []featuresql.ConfigurationsEnvironment{
		{ID: envID, Feature: "f", Key: "only-env", Value: []byte(`"e"`), Created: envCreated, EnvironmentID: environmentID},
	}

	got := MergeConfigs(globals, envs, nil)

	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}

	byKey := map[string]MergedConfigRow{}
	for _, r := range got {
		byKey[r.Key] = r
	}

	if g := byKey["only-global"]; g.ID != globalID || !g.Created.Equal(globalCreated) || g.EnvironmentID != nil {
		t.Errorf("global row = %+v, want ID=%v Created=%v EnvironmentID=nil", g, globalID, globalCreated)
	}
	if e := byKey["only-env"]; e.ID != envID || !e.Created.Equal(envCreated) || e.EnvironmentID == nil || *e.EnvironmentID != environmentID {
		t.Errorf("env row = %+v, want ID=%v Created=%v EnvironmentID=%v", e, envID, envCreated, environmentID)
	}
}

func TestMergeConfigs_EnvOverridesGlobal(t *testing.T) {
	globalID, envID := uuid.New(), uuid.New()
	environmentID := uuid.New()

	globals := []featuresql.ConfigurationsGlobal{
		{ID: globalID, Feature: "f", Key: "k", Value: []byte(`"global"`)},
	}
	envs := []featuresql.ConfigurationsEnvironment{
		{ID: envID, Feature: "f", Key: "k", Value: []byte(`"env"`), EnvironmentID: environmentID},
	}

	got := MergeConfigs(globals, envs, nil)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	r := got[0]
	if string(r.Value) != `"env"` {
		t.Errorf("Value = %s, want \"env\"", r.Value)
	}
	if r.ID != envID {
		t.Errorf("ID = %v, want env ID %v (env override should carry its own ID)", r.ID, envID)
	}
	if r.EnvironmentID == nil || *r.EnvironmentID != environmentID {
		t.Errorf("EnvironmentID = %v, want %v", r.EnvironmentID, environmentID)
	}
}

// TestMergeConfigs_SortedByKey guards against re-introducing the non-deterministic
// map-iteration order in MergeConfigs. The order is load-bearing: MakeHelmConfigMap's
// "is not nestable" check only fires when shorter prefixes are processed before their
// extensions (e.g. "my" before "my.key").
func TestMergeConfigs_SortedByKey(t *testing.T) {
	globals := []featuresql.ConfigurationsGlobal{
		{ID: uuid.New(), Feature: "f", Key: "zeta"},
		{ID: uuid.New(), Feature: "f", Key: "my.key"},
		{ID: uuid.New(), Feature: "f", Key: "alpha"},
		{ID: uuid.New(), Feature: "f", Key: "my"},
	}

	for i := 0; i < 50; i++ {
		got := MergeConfigs(globals, nil, nil)
		want := []string{"alpha", "my", "my.key", "zeta"}
		for j, w := range want {
			if got[j].Key != w {
				t.Fatalf("iter %d: got[%d].Key = %q, want %q (full order: %v)", i, j, got[j].Key, w, keysOf(got))
			}
		}
	}
}

func TestMergeConfigs_IncludeKeysFilter(t *testing.T) {
	globals := []featuresql.ConfigurationsGlobal{
		{ID: uuid.New(), Feature: "f", Key: "keep"},
		{ID: uuid.New(), Feature: "f", Key: "drop"},
	}
	envs := []featuresql.ConfigurationsEnvironment{
		{ID: uuid.New(), Feature: "f", Key: "keep-env", EnvironmentID: uuid.New()},
		{ID: uuid.New(), Feature: "f", Key: "drop-env", EnvironmentID: uuid.New()},
	}

	got := MergeConfigs(globals, envs, []string{"keep", "keep-env"})
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2 (filter dropped %v)", len(got), keysOf(got))
	}
	for _, r := range got {
		if r.Key != "keep" && r.Key != "keep-env" {
			t.Errorf("unexpected key %q in filtered result", r.Key)
		}
	}
}

func keysOf(rows []MergedConfigRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Key
	}
	return out
}

// TestMakeHelmConfigMap_NestingConflictOrderIndependent ensures the
// "is not nestable" check fires regardless of input order. Previously the
// check only triggered when the leaf preceded its nested extension; the
// reverse silently overwrote, which surfaced as the flaky
// HelmValues_InvalidKeyNesting integration test.
func TestMakeHelmConfigMap_NestingConflictOrderIndependent(t *testing.T) {
	leaf := MergedConfigRow{Key: "my", Value: []byte(`"v"`)}
	nested := MergedConfigRow{Key: "my.key", Value: []byte(`"v"`)}

	cases := []struct {
		name string
		in   []MergedConfigRow
	}{
		{"leaf-first", []MergedConfigRow{leaf, nested}},
		{"nested-first", []MergedConfigRow{nested, leaf}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := MakeHelmConfigMap(tc.in)
			if err == nil || !strings.Contains(err.Error(), "is not nestable") {
				t.Errorf("got err=%v, want error containing \"is not nestable\"", err)
			}
		})
	}
}

func TestConfigType_IsValid(t *testing.T) {
	tests := map[string]struct {
		input ConfigType
		valid bool
	}{
		"ConfigTypeString": {
			input: ConfigTypeString,
			valid: true,
		},
		"ConfigTypeInt": {
			input: ConfigTypeInt,
			valid: true,
		},
		"ConfigTypeBool": {
			input: ConfigTypeBool,
			valid: true,
		},
		"ConfigTypeStringArray": {
			input: ConfigTypeStringArray,
			valid: true,
		},
		"ConfigTypeInvalid": {
			input: ConfigType("invalid"),
			valid: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.valid != tc.input.IsValid() {
				t.Errorf("expected %v, got %v", tc.valid, tc.input.IsValid())
			}
		})
	}
}

func TestConfigType_String(t *testing.T) {
	tests := map[string]struct {
		input  ConfigType
		output string
	}{
		"ConfigTypeString": {
			input:  ConfigTypeString,
			output: "string",
		},
		"ConfigTypeInt": {
			input:  ConfigTypeInt,
			output: "int",
		},
		"ConfigTypeBool": {
			input:  ConfigTypeBool,
			output: "bool",
		},
		"ConfigTypeStringArray": {
			input:  ConfigTypeStringArray,
			output: "string_array",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.output != tc.input.String() {
				t.Errorf("expected %q, got %q", tc.output, tc.input.String())
			}
		})
	}
}

func TestConfigType_MarshalJSON(t *testing.T) {
	tests := map[string]struct {
		input  ConfigType
		output string
	}{
		"ConfigTypeString": {
			input:  ConfigTypeString,
			output: `"STRING"`,
		},
		"ConfigTypeInt": {
			input:  ConfigTypeInt,
			output: `"INT"`,
		},
		"ConfigTypeBool": {
			input:  ConfigTypeBool,
			output: `"BOOL"`,
		},
		"ConfigTypeStringArray": {
			input:  ConfigTypeStringArray,
			output: `"STRING_ARRAY"`,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			buf, err := tc.input.MarshalJSON()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tc.output != string(buf) {
				t.Errorf("expected %q, got %q", tc.output, string(buf))
			}
		})
	}
}

func TestConfigType_UnmarshalJSON(t *testing.T) {
	tests := map[string]struct {
		input  string
		output ConfigType
		valid  bool
	}{
		"ConfigTypeString": {
			input:  `"STRING"`,
			output: ConfigTypeString,
			valid:  true,
		},
		"ConfigTypeInt": {
			input:  `"INT"`,
			output: ConfigTypeInt,
			valid:  true,
		},
		"ConfigTypeBool": {
			input:  `"BOOL"`,
			output: ConfigTypeBool,
			valid:  true,
		},
		"ConfigTypeStringArray": {
			input:  `"STRING_ARRAY"`,
			output: ConfigTypeStringArray,
			valid:  true,
		},
		"ConfigTypeInvalid": {
			input:  `"INVALID"`,
			output: ConfigType("invalid"),
			valid:  true,
		},
		"ConfigTypeInvalidJSON": {
			input:  `"INVALID'`,
			output: "",
			valid:  false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var output ConfigType
			err := output.UnmarshalJSON([]byte(tc.input))
			if tc.valid != (err == nil) {
				t.Errorf("expected %v, got %v", tc.valid, err == nil)
			}
			if tc.valid && tc.output != output {
				t.Errorf("expected %q, got %q", tc.output, output)
			}
		})
	}
}

func TestDependencies_FindMissing(t *testing.T) {
	tests := map[string]struct {
		dep      Dependencies
		features []string
		want     []string
	}{
		"empty": {
			dep:  Dependencies{},
			want: []string{},
		},
		"any of": {
			dep: Dependencies{
				{
					AnyOf: []string{"foo", "bar"},
				},
			},
			features: []string{"foo"},
			want:     []string{},
		},
		"all of": {
			dep: Dependencies{
				{
					AllOf: []string{"foo", "bar"},
				},
			},
			features: []string{"foo", "bar"},
			want:     []string{},
		},
		"all of and any of": {
			dep: Dependencies{
				{
					AllOf: []string{"foo", "bar"},
					AnyOf: []string{"baz", "qux"},
				},
			},
			features: []string{"foo", "bar", "baz"},
			want:     []string{},
		},
		"all of and any of, not satisfied": {
			dep: Dependencies{
				{
					AllOf: []string{"foo", "bar"},
					AnyOf: []string{"baz", "qux"},
				},
			},
			features: []string{"foo", "bar"},
			want:     []string{"baz", "qux"},
		},
		"all of, not satisfied": {
			dep: Dependencies{
				{
					AllOf: []string{"foo", "bar"},
				},
			},
			features: []string{"foo", "baz"},
			want:     []string{"bar"},
		},
		"all of, not satisfied, no features": {
			dep: Dependencies{
				{
					AllOf: []string{"foo", "bar"},
				},
			},
			features: []string{},
			want:     []string{"foo", "bar"},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := tt.dep.FindMissing(tt.features); !cmp.Equal(tt.want, got) {
				t.Errorf("diff -want +got:\n%v", cmp.Diff(tt.want, got))
			}
		})
	}
}
