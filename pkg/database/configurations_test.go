package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
)

func TestHelmConfigMap(t *testing.T) {
	jsonify := func(v any) pgtype.JSONB {
		b, _ := json.Marshal(v)
		return pgtype.JSONB{
			Bytes:  b,
			Status: pgtype.Present,
		}
	}
	tests := map[string]struct {
		input    []gensql.EnvConfigRow
		expected map[string]any
	}{
		"empty": {
			input:    nil,
			expected: make(map[string]any),
		},
		"single_level": {
			input: []gensql.EnvConfigRow{
				{
					Key:   "test1",
					Value: jsonify("value1"),
				},
				{
					Key:   "test2",
					Value: jsonify("value2"),
				},
			},
			expected: map[string]any{
				"test1": jsonify("value1"),
				"test2": jsonify("value2"),
			},
		},
		"multi_level": {
			input: []gensql.EnvConfigRow{
				{
					Key:   "test.a",
					Value: jsonify("value_a"),
				},
				{
					Key:   "test.b",
					Value: jsonify("value_b"),
				},
			},
			expected: map[string]any{
				"test": map[string]any{
					"a": jsonify("value_a"),
					"b": jsonify("value_b"),
				},
			},
		},
		"escaped dots": {
			input: []gensql.EnvConfigRow{
				{
					Key:   "test.a",
					Value: jsonify("value_a"),
				},
				{
					Key:   "test.b\\.escaped",
					Value: jsonify("value_b"),
				},
			},
			expected: map[string]any{
				"test": map[string]any{
					"a":         jsonify("value_a"),
					"b.escaped": jsonify("value_b"),
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			retval, err := makeHelmConfigMap(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			if !cmp.Equal(retval, tc.expected) {
				t.Error(cmp.Diff(retval, tc.expected))
			}
		})
	}
}

func TestRepo_ConfigGet(t *testing.T) {
	id := uuid.New()
	created := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	description := "description"
	q := `INSERT INTO configurations_global (
		id, feature, key, value, description, secret, created
	) VALUES (
		'%s', 'feature3', 'my.key', '"stringval"', '%s', true, '%s'
	)`
	repo := newTestRepo(t, fmt.Sprintf(q, id, description, created.Format(time.RFC3339)))
	defer repo.Close()

	got, err := repo.ConfigGet(context.Background(), "feature3")
	if err != nil {
		t.Fatal(err)
	}

	want := []*model.GlobalConfiguration{
		{
			ID:          id,
			FeatureName: "feature3",
			Key:         "my.key",
			Value:       []byte(`"stringval"`),
			Created:     created,
			Secret:      true,
		},
	}

	if !cmp.Equal(want, got) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got))
	}
}

func TestRepo_ConfigGetForEnv(t *testing.T) {
	id := uuid.New()
	envid := uuid.New()
	tenantid := uuid.New()
	created := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	description := "description"

	q1 := `INSERT INTO tenants (id, name) VALUES ('%s', 'tenant1')`
	q2 := `INSERT INTO environments (id, tenant_id, name, kind) VALUES ('%s', '%s', 'env1', 'tenant')`
	q3 := `INSERT INTO configurations_environment (
		id, environment_id, feature, key, value, description, secret, created
	) VALUES (
		'%s', '%s', 'feature3', 'my.key', '"stringval"', '%s', true, '%s'
	)`

	repo := newTestRepo(t,
		fmt.Sprintf(q1, tenantid),
		fmt.Sprintf(q2, envid, tenantid),
		fmt.Sprintf(q3, id, envid, description, created.Format(time.RFC3339)),
	)
	defer repo.Close()

	got, err := repo.ConfigGetForEnv(context.Background(), "feature3", envid)
	if err != nil {
		t.Fatal(err)
	}

	want := []*model.EnvConfiguration{
		{
			ID:            id,
			EnvironmentID: envid,
			FeatureName:   "feature3",
			Key:           "my.key",
			Value:         []byte(`"stringval"`),
			Created:       created,
			Secret:        true,
		},
	}

	if !cmp.Equal(want, got) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got))
	}
}

func TestRepo_ConfigCreate_Environment(t *testing.T) {
	envid := uuid.New()
	tenantid := uuid.New()
	q1 := `INSERT INTO tenants (id, name) VALUES ('%s', 'tenant1')`
	q2 := `INSERT INTO environments (id, tenant_id, name, kind) VALUES ('%s', '%s', 'env1', 'tenant')`
	repo := newTestRepo(t, fmt.Sprintf(q1, tenantid), fmt.Sprintf(q2, envid, tenantid))
	defer repo.Close()

	config := model.NewConfiguration{
		EnvironmentID: &envid,
		Feature:       "feature5",
		Description:   stringToPtr("description"),
		Key:           "my.key",
		Value:         []byte(`"stringval"`),
		Secret:        true,
	}
	got, err := repo.ConfigCreate(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}

	want := &model.EnvConfiguration{
		EnvironmentID: *config.EnvironmentID,
		FeatureName:   config.Feature,
		Key:           config.Key,
		Value:         config.Value,
		Secret:        config.Secret,
	}

	opts := cmpopts.IgnoreFields(model.EnvConfiguration{}, "ID", "Created")
	if !cmp.Equal(want, got, opts) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got, opts))
	}
}

func TestRepo_ConfigCreate_Global(t *testing.T) {
	repo := newTestRepo(t)
	defer repo.Close()
	envid := uuid.Nil
	config := model.NewConfiguration{
		EnvironmentID: &envid,
		Feature:       "feature5",
		Description:   stringToPtr("description"),
		Key:           "my.key",
		Value:         []byte(`"stringval"`),
		Secret:        true,
	}
	got, err := repo.ConfigCreate(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}

	want := &model.GlobalConfiguration{
		FeatureName: config.Feature,
		Key:         config.Key,
		Value:       config.Value,
		Secret:      config.Secret,
	}

	opts := cmpopts.IgnoreFields(model.GlobalConfiguration{}, "ID", "Created")
	if !cmp.Equal(want, got, opts) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got, opts))
	}
}

func TestRepo_ConfigUpdate_Global(t *testing.T) {
	repo := newTestRepo(t)
	defer repo.Close()

	config := model.NewConfiguration{
		Feature:     "feature5",
		Description: stringToPtr("description"),
		Key:         "my.key",
		Value:       []byte(`"stringval"`),
		Secret:      true,
	}
	// Create
	got, err := repo.ConfigCreate(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}

	got, err = repo.ConfigUpdate(context.Background(), got.(*model.GlobalConfiguration).ID, model.UpdateConfiguration{
		Value: []byte(`"newval"`),
	})
	if err != nil {
		t.Fatal(err)
	}

	want := &model.GlobalConfiguration{
		FeatureName: config.Feature,
		Key:         config.Key,
		Value:       []byte(`"newval"`),
		Secret:      config.Secret,
	}

	opts := cmpopts.IgnoreFields(model.GlobalConfiguration{}, "ID", "Created")
	if !cmp.Equal(want, got, opts) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got, opts))
	}
}

func TestRepo_ConfigDelete(t *testing.T) {
	r := newTestRepo(t)
	defer r.Close()

	config := model.NewConfiguration{
		Feature: "feature9",
		Key:     "my.key",
		Value:   []byte(`"stringval"`),
	}
	// Create
	got, err := r.ConfigCreate(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}

	gotID := got.(*model.GlobalConfiguration).ID
	err = r.ConfigDelete(context.Background(), gotID)
	if err != nil {
		t.Fatal(err)
	}
	confs, err := r.ConfigGet(context.Background(), config.Feature)
	if err != nil {
		return
	}
	found := false
	for _, conf := range confs {
		if conf.ID == gotID {
			found = true
		}
	}
	if found {
		t.Errorf("got: %v, want %v", found, false)
	}
}

func TestRepo_HelmValues_OK(t *testing.T) {
	envid := uuid.New()
	tenantid := uuid.New()
	q1 := `INSERT INTO tenants (id, name) VALUES ('%s', 'tenant1')`
	q2 := `INSERT INTO environments (id, tenant_id, name, kind) VALUES ('%s', '%s', 'env1', 'tenant')`
	r := newTestRepo(t, fmt.Sprintf(q1, tenantid), fmt.Sprintf(q2, envid, tenantid))
	defer r.Close()

	feature := model.Feature{
		Name: "feature5",
		FeatureYAML: model.FeatureYAML{
			Values: model.Values{
				"my.key": model.Value{
					Config: &model.Config{
						Type:   model.ConfigTypeString,
						Secret: true,
					},
				},
			},
		},
	}

	config := model.NewConfiguration{
		EnvironmentID: &envid,
		Feature:       feature.Name,
		Key:           "my.key",
		Value:         []byte(`"stringval"`),
		Secret:        true,
	}
	// Create
	_, err := r.ConfigCreate(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}

	got, err := r.HelmValues(context.Background(), &feature, envid)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]any{
		"my": map[string]any{
			"key": pgtype.JSONB{Bytes: []byte(`"stringval"`), Status: pgtype.Present},
		},
	}

	if !cmp.Equal(want, got) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got))
	}
}

func TestRepo_HelmValues_MissingRequiredField(t *testing.T) {
	envid := uuid.New()
	tenantid := uuid.New()
	q1 := `INSERT INTO tenants (id, name) VALUES ('%s', 'tenant1')`
	q2 := `INSERT INTO environments (id, tenant_id, name, kind) VALUES ('%s', '%s', 'env1', 'tenant')`
	r := newTestRepo(t, fmt.Sprintf(q1, tenantid), fmt.Sprintf(q2, envid, tenantid))
	defer r.Close()

	feature := model.Feature{
		Name: "feature5",
		FeatureYAML: model.FeatureYAML{
			Values: model.Values{
				"my.key": model.Value{
					Config: &model.Config{
						Type:   model.ConfigTypeString,
						Secret: true,
					},
				},
				"no.key": model.Value{
					Config: &model.Config{
						Type:   model.ConfigTypeString,
						Secret: true,
					},
					Required: true,
				},
			},
		},
	}

	config := model.NewConfiguration{
		EnvironmentID: &envid,
		Feature:       "feature5",
		Key:           "my.key",
		Value:         []byte(`"stringval"`),
		Secret:        true,
	}
	// Create
	_, err := r.ConfigCreate(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}

	_, err = r.HelmValues(context.Background(), &feature, envid)
	if !errors.Is(err, &ErrMissingRequiredFields{}) {
		t.Errorf("got: %v, want ErrMissingRequiredFields", err)
	}
}

func TestRepo_HelmValues_InvaldKeyNesting(t *testing.T) {
	envid := uuid.New()
	tenantid := uuid.New()
	q1 := `INSERT INTO tenants (id, name) VALUES ('%s', 'tenant1')`
	q2 := `INSERT INTO environments (id, tenant_id, name, kind) VALUES ('%s', '%s', 'env1', 'tenant')`
	r := newTestRepo(t, fmt.Sprintf(q1, tenantid), fmt.Sprintf(q2, envid, tenantid))
	defer r.Close()

	config := model.NewConfiguration{
		Feature: "feature5",
		Key:     "my.key",
		Value:   []byte(`"stringval"`),
		Secret:  true,
	}
	config2 := model.NewConfiguration{
		Feature: "feature5",
		Key:     "my",
		Value:   []byte(`15`),
		Secret:  true,
	}
	// Create
	_, err := r.ConfigCreate(context.Background(), config2)
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.ConfigCreate(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}

	_, err = r.HelmValues(context.Background(), &model.Feature{Name: "feature5"}, envid)
	if err == nil || !strings.HasSuffix(err.Error(), "is not nestable") {
		t.Errorf("got: %v, want \"key `key` is not nestable\"", err)
	}
}

func TestRepo_HelmValues_WithMappingValues(t *testing.T) {
	envid := uuid.New()
	mgmtID := uuid.New()
	tenantid := uuid.New()
	q1 := `INSERT INTO tenants (id, name) VALUES ('%s', 'tenant1')`
	q2 := `INSERT INTO environments (id, tenant_id, name, kind) VALUES ('%s', '%s', 'management', 'management')`
	q3 := `INSERT INTO environments (id, tenant_id, name, kind) VALUES ('%s', '%s', 'env1', 'tenant')`
	r := newTestRepo(t, fmt.Sprintf(q1, tenantid), fmt.Sprintf(q2, mgmtID, tenantid), fmt.Sprintf(q3, envid, tenantid))
	defer r.Close()

	vali := []struct {
		EnvID  uuid.UUID
		Key    string
		Value  json.RawMessage
		Secret bool
	}{
		{mgmtID, "project_id", json.RawMessage(`"my-project"`), false},
		{envid, "project_id", json.RawMessage(`"env-project"`), false},
		{envid, "some_secret", json.RawMessage(`"hideme"`), true},
	}

	f := model.Feature{
		Name: "feature5",
		FeatureYAML: model.FeatureYAML{
			Values: model.Values{
				"names.tenant": model.Value{
					Computed: &model.Computed{
						Template: "{{ .Tenant.Name }}",
					},
				},
				"names.environment": model.Value{
					Computed: &model.Computed{
						Template: "{{ .Env.name }}",
					},
				},
				"kind": model.Value{
					Computed: &model.Computed{
						Template: "{{ .Kind }}",
					},
				},
				"projects.env": model.Value{
					Computed: &model.Computed{
						Template: "{{ .Env.project_id }}",
					},
				},
				"projects.mgmt": model.Value{
					Computed: &model.Computed{
						Template: "{{ .Management.project_id }}",
					},
				},
			},
		},
	}

	for _, v := range vali {
		err := r.EnvironmentValueStore(context.Background(), v.EnvID, v.Key, v.Value, v.Secret)
		if err != nil {
			t.Fatal(err)
		}
	}

	got, err := r.HelmValues(context.Background(), &f, envid)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]any{
		"kind":     "tenant",
		"names":    map[string]any{"environment": "env1", "tenant": "tenant1"},
		"projects": map[string]any{"env": "env-project", "mgmt": "my-project"},
	}

	if !cmp.Equal(want, got) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got))
	}
}

func TestRepo_HelmValues_WithIgnoredKeys_Ignored(t *testing.T) {
	envid := uuid.New()
	tenantid := uuid.New()
	q1 := `INSERT INTO tenants (id, name) VALUES ('%s', 'tenant1')`
	q2 := `INSERT INTO environments (id, tenant_id, name, kind) VALUES ('%s', '%s', 'env1', 'onprem')`
	r := newTestRepo(t, fmt.Sprintf(q1, tenantid), fmt.Sprintf(q2, envid, tenantid))
	defer r.Close()

	feature := model.Feature{
		Name: "feature6",
		FeatureYAML: model.FeatureYAML{
			Values: model.Values{
				"my.key": model.Value{
					Config: &model.Config{
						Type:   model.ConfigTypeString,
						Secret: true,
					},
				},
				"ignore.key": model.Value{
					Required: true,
					IgnoreKind: []model.EnvironmentKind{
						model.EnvironmentKindOnprem,
					},
					Config: &model.Config{
						Type:   model.ConfigTypeString,
						Secret: true,
					},
				},
			},
		},
	}

	config := model.NewConfiguration{
		EnvironmentID: &envid,
		Feature:       feature.Name,
		Key:           "my.key",
		Value:         []byte(`"stringval"`),
		Secret:        true,
	}
	_, err := r.ConfigCreate(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}

	globConfig := model.NewConfiguration{
		Feature: feature.Name,
		Key:     "ignore.key",
		Value:   []byte(`"ignore"`),
		Secret:  true,
	}
	_, err = r.ConfigCreate(context.Background(), globConfig)
	if err != nil {
		t.Fatal(err)
	}

	got, err := r.HelmValues(context.Background(), &feature, envid)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]any{
		"my": map[string]any{
			"key": pgtype.JSONB{Bytes: []byte(`"stringval"`), Status: pgtype.Present},
		},
	}

	if !cmp.Equal(want, got) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got))
	}
}

func TestRepo_HelmValues_WithIgnoredKeys_NotIgnored(t *testing.T) {
	envid := uuid.New()
	tenantid := uuid.New()
	q1 := `INSERT INTO tenants (id, name) VALUES ('%s', 'tenant1')`
	q2 := `INSERT INTO environments (id, tenant_id, name, kind) VALUES ('%s', '%s', 'env1', 'tenant')`
	r := newTestRepo(t, fmt.Sprintf(q1, tenantid), fmt.Sprintf(q2, envid, tenantid))
	defer r.Close()

	feature := model.Feature{
		Name: "feature6",
		FeatureYAML: model.FeatureYAML{
			Values: model.Values{
				"my.key": model.Value{
					Config: &model.Config{
						Type:   model.ConfigTypeString,
						Secret: true,
					},
				},
				"ignore.key": model.Value{
					Required: true,
					IgnoreKind: []model.EnvironmentKind{
						model.EnvironmentKindOnprem,
					},
					Config: &model.Config{
						Type:   model.ConfigTypeString,
						Secret: true,
					},
				},
			},
		},
	}

	config := model.NewConfiguration{
		EnvironmentID: &envid,
		Feature:       feature.Name,
		Key:           "my.key",
		Value:         []byte(`"stringval"`),
		Secret:        true,
	}
	_, err := r.ConfigCreate(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}

	globConfig := model.NewConfiguration{
		Feature: feature.Name,
		Key:     "ignore.key",
		Value:   []byte(`"ignore"`),
		Secret:  true,
	}
	_, err = r.ConfigCreate(context.Background(), globConfig)
	if err != nil {
		t.Fatal(err)
	}

	got, err := r.HelmValues(context.Background(), &feature, envid)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]any{
		"my": map[string]any{
			"key": pgtype.JSONB{Bytes: []byte(`"stringval"`), Status: pgtype.Present},
		},
		"ignore": map[string]any{
			"key": pgtype.JSONB{Bytes: []uint8(`"ignore"`), Status: pgtype.Present},
		},
	}

	if !cmp.Equal(want, got) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got))
	}
}
