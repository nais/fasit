//go:build integration_test

package feature

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/database/dbtest"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/errs"
	"github.com/nais/fasit/internal/feature/featuresql"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"k8s.io/utils/ptr"
)

func TestHelmConfigMap(t *testing.T) {
	jsonify := func(v any) json.RawMessage {
		b, _ := json.Marshal(v)
		return b
	}
	tests := map[string]struct {
		input    []featuresql.ConfigForEnvironmentFilteredByKeysRow
		expected map[string]any
	}{
		"empty": {
			input:    nil,
			expected: make(map[string]any),
		},
		"single_level": {
			input: []featuresql.ConfigForEnvironmentFilteredByKeysRow{
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
			input: []featuresql.ConfigForEnvironmentFilteredByKeysRow{
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
			input: []featuresql.ConfigForEnvironmentFilteredByKeysRow{
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
	ctx, pool := setup(t)

	id := uuid.New()
	created := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	description := "description"
	q := `INSERT INTO configurations_global (
		id, feature, key, value, description, secret, created
	) VALUES (
		'%s', 'feature3', 'my.key', '"stringval"', '%s', true, '%s'
	)`

	execQuery(ctx, t, pool, fmt.Sprintf(q, id, description, created.Format(time.RFC3339)))

	got, err := ConfigGet(ctx, "feature3")
	if err != nil {
		t.Fatal(err)
	}

	want := []*model.Configuration{
		{
			ID:        id,
			GraphVars: model.ConfigurationGraphVars{FeatureName: "feature3"},
			Key:       "my.key",
			Content:   []byte(`"stringval"`),
			Created:   created,
			Source:    model.ConfigSourceGlobal,
		},
	}

	if !cmp.Equal(want, got) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got))
	}
}

func TestRepo_ConfigCreate_Environment(t *testing.T) {
	ctx, pool := setup(t)

	envid := uuid.New()
	tenantid := uuid.New()
	q1 := `INSERT INTO tenants (id, name) VALUES ('%s', 'tenant1')`
	q2 := `INSERT INTO environments (id, tenant_id, name, kind) VALUES ('%s', '%s', 'env1', 'tenant')`

	execQuery(ctx, t, pool, fmt.Sprintf(q1, tenantid), fmt.Sprintf(q2, envid, tenantid))

	config := model.NewConfiguration{
		EnvironmentID: &envid,
		Feature:       "feature5",
		Description:   ptr.To("description"),
		Key:           "my.key",
		Value:         []byte(`"stringval"`),
		Secret:        true,
	}
	got, err := ConfigCreate(ctx, config)
	if err != nil {
		t.Fatal(err)
	}

	want := &model.Configuration{
		GraphVars: model.ConfigurationGraphVars{
			FeatureName:   "feature5",
			EnvironmentID: &envid,
		},
		Key:     config.Key,
		Content: config.Value,
		Source:  model.ConfigSourceEnv,
	}

	opts := cmpopts.IgnoreFields(model.Configuration{}, "ID", "Created")
	if !cmp.Equal(want, got, opts) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got, opts))
	}
}

func TestRepo_ConfigCreate_Global(t *testing.T) {
	ctx, _ := setup(t)

	envid := uuid.Nil
	config := model.NewConfiguration{
		EnvironmentID: &envid,
		Feature:       "feature5",
		Description:   ptr.To("description"),
		Key:           "my.key",
		Value:         []byte(`"stringval"`),
		Secret:        true,
	}
	got, err := ConfigCreate(ctx, config)
	if err != nil {
		t.Fatal(err)
	}

	want := &model.Configuration{
		GraphVars: model.ConfigurationGraphVars{
			FeatureName: "feature5",
		},
		Key:     config.Key,
		Content: config.Value,
		Source:  model.ConfigSourceGlobal,
	}

	opts := cmpopts.IgnoreFields(model.Configuration{}, "ID", "Created")
	if !cmp.Equal(want, got, opts) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got, opts))
	}
}

func TestRepo_ConfigUpdate_Global(t *testing.T) {
	ctx, _ := setup(t)

	config := model.NewConfiguration{
		Feature:     "feature5",
		Description: ptr.To("description"),
		Key:         "my.key",
		Value:       []byte(`"stringval"`),
		Secret:      true,
	}
	// Create
	got, err := ConfigCreate(ctx, config)
	if err != nil {
		t.Fatal(err)
	}

	got, err = ConfigUpdate(ctx, got.ID, model.UpdateConfiguration{
		Value: []byte(`"newval"`),
	})
	if err != nil {
		t.Fatal(err)
	}

	want := &model.Configuration{
		GraphVars: model.ConfigurationGraphVars{FeatureName: config.Feature},
		Key:       config.Key,
		Content:   []byte(`"newval"`),
		Source:    model.ConfigSourceGlobal,
	}

	opts := cmpopts.IgnoreFields(model.Configuration{}, "ID", "Created")
	if !cmp.Equal(want, got, opts) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got, opts))
	}
}

func TestRepo_ConfigDelete(t *testing.T) {
	ctx, _ := setup(t)

	config := model.NewConfiguration{
		Feature: "feature9",
		Key:     "my.key",
		Value:   []byte(`"stringval"`),
	}
	// Create
	got, err := ConfigCreate(ctx, config)
	if err != nil {
		t.Fatal(err)
	}

	gotID := got.ID
	err = ConfigDelete(ctx, gotID)
	if err != nil {
		t.Fatal(err)
	}
	confs, err := ConfigGet(ctx, config.Feature)
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
	ctx, pool := setup(t)

	envid := uuid.New()
	tenantid := uuid.New()
	q1 := `INSERT INTO tenants (id, name) VALUES ('%s', 'tenant1')`
	q2 := `INSERT INTO environments (id, tenant_id, name, kind) VALUES ('%s', '%s', 'env1', 'tenant')`

	execQuery(ctx, t, pool, fmt.Sprintf(q1, tenantid), fmt.Sprintf(q2, envid, tenantid))

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
	_, err := ConfigCreate(ctx, config)
	if err != nil {
		t.Fatal(err)
	}

	got, err := HelmValues(ctx, &feature, envid)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]any{
		"fasit": map[string]any{
			"env":    map[string]string{"kind": "tenant", "name": "env1"},
			"tenant": map[string]string{"name": "tenant1"},
		},
		"my": map[string]any{
			"key": json.RawMessage(`"stringval"`),
		},
	}

	if !cmp.Equal(want, got) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got))
	}
}

func TestRepo_HelmValues_MissingRequiredField(t *testing.T) {
	ctx, pool := setup(t)

	envid := uuid.New()
	tenantid := uuid.New()
	q1 := `INSERT INTO tenants (id, name) VALUES ('%s', 'tenant1')`
	q2 := `INSERT INTO environments (id, tenant_id, name, kind) VALUES ('%s', '%s', 'env1', 'tenant')`
	execQuery(ctx, t, pool, fmt.Sprintf(q1, tenantid), fmt.Sprintf(q2, envid, tenantid))

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
	_, err := ConfigCreate(ctx, config)
	if err != nil {
		t.Fatal(err)
	}

	_, err = HelmValues(ctx, &feature, envid)
	if !errors.Is(err, &errs.ErrMissingRequiredFields{}) {
		t.Errorf("got: %v, want ErrMissingRequiredFields", err)
	}
}

func TestRepo_HelmValues_InvaldKeyNesting(t *testing.T) {
	ctx, pool := setup(t)
	envid := uuid.New()
	tenantid := uuid.New()
	q1 := `INSERT INTO tenants (id, name) VALUES ('%s', 'tenant1')`
	q2 := `INSERT INTO environments (id, tenant_id, name, kind) VALUES ('%s', '%s', 'env1', 'tenant')`
	execQuery(ctx, t, pool, fmt.Sprintf(q1, tenantid), fmt.Sprintf(q2, envid, tenantid))

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
	_, err := ConfigCreate(ctx, config2)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ConfigCreate(ctx, config)
	if err != nil {
		t.Fatal(err)
	}

	feature := &model.Feature{
		Name: "feature5",
		FeatureYAML: model.FeatureYAML{
			Values: model.Values{
				"my.key": model.Value{
					Config: &model.Config{
						Type: model.ConfigTypeString,
					},
				},
				"my": model.Value{
					Config: &model.Config{
						Type: model.ConfigTypeString,
					},
				},
			},
		},
	}
	_, err = HelmValues(ctx, feature, envid)
	if err == nil || !strings.HasSuffix(err.Error(), "is not nestable") {
		t.Errorf("got: %v, want \"key `key` is not nestable\"", err)
	}
}

func TestRepo_HelmValues_WithMappingValues(t *testing.T) {
	ctx, pool := setup(t)
	envid := uuid.New()
	mgmtID := uuid.New()
	tenantid := uuid.New()
	q1 := `INSERT INTO tenants (id, name) VALUES ('%s', 'tenant1')`
	q2 := `INSERT INTO environments (id, tenant_id, name, kind) VALUES ('%s', '%s', 'management', 'management')`
	q3 := `INSERT INTO environments (id, tenant_id, name, kind) VALUES ('%s', '%s', 'env1', 'tenant')`
	execQuery(ctx, t, pool, fmt.Sprintf(q1, tenantid), fmt.Sprintf(q2, mgmtID, tenantid), fmt.Sprintf(q3, envid, tenantid))

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
		q := `INSERT INTO environment_values(
	"environment_id",
	"key",
	"value",
	"secret")
VALUES (
	'%[1]s',
	'%[2]s',
	'%[3]s',
	%[4]v)
ON CONFLICT (
	"environment_id",
	"key")
	DO UPDATE SET
		"value" = '%[3]s',
		"secret" = %[4]v`

		execQuery(ctx, t, pool, fmt.Sprintf(q, v.EnvID, v.Key, v.Value, v.Secret))
	}

	got, err := HelmValues(ctx, &f, envid)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]any{
		"fasit": map[string]any{
			"env":    map[string]string{"kind": "tenant", "name": "env1"},
			"tenant": map[string]string{"name": "tenant1"},
		},
		"kind":     "tenant",
		"names":    map[string]any{"environment": "env1", "tenant": "tenant1"},
		"projects": map[string]any{"env": "env-project", "mgmt": "my-project"},
	}

	if !cmp.Equal(want, got) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got))
	}
}

func TestRepo_HelmValues_WithIgnoredKeys_Ignored(t *testing.T) {
	ctx, pool := setup(t)
	envid := uuid.New()
	tenantid := uuid.New()
	q1 := `INSERT INTO tenants (id, name) VALUES ('%s', 'tenant1')`
	q2 := `INSERT INTO environments (id, tenant_id, name, kind) VALUES ('%s', '%s', 'env1', 'onprem')`
	execQuery(ctx, t, pool, fmt.Sprintf(q1, tenantid), fmt.Sprintf(q2, envid, tenantid))

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
		Value:         json.RawMessage(`"stringval"`),
		Secret:        true,
	}
	_, err := ConfigCreate(ctx, config)
	if err != nil {
		t.Fatal(err)
	}

	globConfig := model.NewConfiguration{
		Feature: feature.Name,
		Key:     "ignore.key",
		Value:   []byte(`"ignore"`),
		Secret:  true,
	}
	_, err = ConfigCreate(ctx, globConfig)
	if err != nil {
		t.Fatal(err)
	}

	got, err := HelmValues(ctx, &feature, envid)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]any{
		"fasit": map[string]any{
			"env":    map[string]string{"kind": "onprem", "name": "env1"},
			"tenant": map[string]string{"name": "tenant1"},
		},
		"my": map[string]any{
			"key": json.RawMessage(`"stringval"`),
		},
	}

	if !cmp.Equal(want, got) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got))
	}
}

func TestRepo_HelmValues_WithIgnoredKeys_NotIgnored(t *testing.T) {
	ctx, pool := setup(t)
	envid := uuid.New()
	tenantid := uuid.New()
	q1 := `INSERT INTO tenants (id, name) VALUES ('%s', 'tenant1')`
	q2 := `INSERT INTO environments (id, tenant_id, name, kind) VALUES ('%s', '%s', 'env1', 'tenant')`
	execQuery(ctx, t, pool, fmt.Sprintf(q1, tenantid), fmt.Sprintf(q2, envid, tenantid))

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
	_, err := ConfigCreate(ctx, config)
	if err != nil {
		t.Fatal(err)
	}

	globConfig := model.NewConfiguration{
		Feature: feature.Name,
		Key:     "ignore.key",
		Value:   []byte(`"ignore"`),
		Secret:  true,
	}
	_, err = ConfigCreate(ctx, globConfig)
	if err != nil {
		t.Fatal(err)
	}

	got, err := HelmValues(ctx, &feature, envid)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]any{
		"fasit": map[string]any{
			"env":    map[string]string{"kind": "tenant", "name": "env1"},
			"tenant": map[string]string{"name": "tenant1"},
		},
		"my": map[string]any{
			"key": json.RawMessage(`"stringval"`),
		},
		"ignore": map[string]any{
			"key": json.RawMessage(`"ignore"`),
		},
	}

	if !cmp.Equal(want, got) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got))
	}

	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}

	expectedJSON := `{"fasit":{"env":{"kind":"tenant","name":"env1"},"tenant":{"name":"tenant1"}},"ignore":{"key":"ignore"},"my":{"key":"stringval"}}`

	if !cmp.Equal(string(b), expectedJSON) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(expectedJSON, string(b)))
	}
}

func setup(t *testing.T) (context.Context, *pgxpool.Pool) {
	pool := setupPostgres(t)
	log, _ := test.NewNullLogger()
	ctx := Register(context.Background(), pool)
	ctx = audit.Register(ctx, pool, log)
	ctx = environment.Register(ctx, pool)

	return ctx, pool
}

func setupPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn, cleanup := dbtest.DockerSQLPool(context.Background())

	log := logrus.New()
	log.Out = io.Discard
	ctx := context.Background()

	pool, closers, err := database.NewConnPool(ctx, dsn, log)
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	t.Cleanup(func() {
		cleanup()
		_ = closers.Close()
	})

	return pool
}

func execQuery(ctx context.Context, t *testing.T, pool *pgxpool.Pool, queries ...string) {
	t.Helper()

	for _, q := range queries {
		if _, err := pool.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
}
