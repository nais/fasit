package feature

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/fasit/internal/feature/featuresql"
)

func TestMergeConfigs_PreservesIDAndCreated(t *testing.T) {
	globalID := uuid.New()
	envID := uuid.New()
	environmentID := uuid.New()
	globalCreated := pgtype.Timestamptz{Time: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}
	envCreated := pgtype.Timestamptz{Time: time.Date(2025, 2, 2, 0, 0, 0, 0, time.UTC), Valid: true}

	globals := []featuresql.ConfigurationsGlobal{
		{ID: globalID, Feature: "f", Key: "only-global", Value: []byte(`"g"`), Created: globalCreated},
	}
	envs := []featuresql.ConfigurationsEnvironment{
		{ID: envID, Feature: "f", Key: "only-env", Value: []byte(`"e"`), Created: envCreated, EnvironmentID: environmentID},
	}

	got := mergeConfigs(globals, envs, nil)

	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}

	byKey := map[string]mergedConfigRow{}
	for _, r := range got {
		byKey[r.Key] = r
	}

	if g := byKey["only-global"]; g.ID != globalID || !g.Created.Time.Equal(globalCreated.Time) || g.EnvironmentID != nil {
		t.Errorf("global row = %+v, want ID=%v Created=%v EnvironmentID=nil", g, globalID, globalCreated.Time)
	}
	if e := byKey["only-env"]; e.ID != envID || !e.Created.Time.Equal(envCreated.Time) || e.EnvironmentID == nil || *e.EnvironmentID != environmentID {
		t.Errorf("env row = %+v, want ID=%v Created=%v EnvironmentID=%v", e, envID, envCreated.Time, environmentID)
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

	got := mergeConfigs(globals, envs, nil)
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
// map-iteration order in mergeConfigs. The order is load-bearing: makeHelmConfigMap's
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
		got := mergeConfigs(globals, nil, nil)
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

	got := mergeConfigs(globals, envs, []string{"keep", "keep-env"})
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2 (filter dropped %v)", len(got), keysOf(got))
	}
	for _, r := range got {
		if r.Key != "keep" && r.Key != "keep-env" {
			t.Errorf("unexpected key %q in filtered result", r.Key)
		}
	}
}

func keysOf(rows []mergedConfigRow) []string {
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
	leaf := mergedConfigRow{Key: "my", Value: []byte(`"v"`)}
	nested := mergedConfigRow{Key: "my.key", Value: []byte(`"v"`)}

	cases := []struct {
		name string
		in   []mergedConfigRow
	}{
		{"leaf-first", []mergedConfigRow{leaf, nested}},
		{"nested-first", []mergedConfigRow{nested, leaf}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := makeHelmConfigMap(tc.in)
			if err == nil || !strings.Contains(err.Error(), "is not nestable") {
				t.Errorf("got err=%v, want error containing \"is not nestable\"", err)
			}
		})
	}
}
