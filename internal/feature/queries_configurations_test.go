//go:build integration_test

package feature

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/errs"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestHelmConfigMap(t *testing.T) {
	jsonify := func(v any) json.RawMessage {
		b, _ := json.Marshal(v)
		return b
	}
	tests := map[string]struct {
		input    []MergedConfigRow
		expected map[string]any
	}{
		"empty": {
			input:    nil,
			expected: make(map[string]any),
		},
		"single_level": {
			input: []MergedConfigRow{
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
			input: []MergedConfigRow{
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
			input: []MergedConfigRow{
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
			retval, err := MakeHelmConfigMap(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			if !cmp.Equal(retval, tc.expected) {
				t.Error(cmp.Diff(retval, tc.expected))
			}
		})
	}
}

func TestConfig(t *testing.T) {
	ctx := context.Background()
	container, dsn := startPostgresql(ctx, t)

	t.Run("GetGlobalConfig", func(t *testing.T) {
		pool := newPool(ctx, t, container, dsn)
		ctx = setupContext(pool)

		id := uuid.New()
		created := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
		description := "description"
		q := `INSERT INTO configurations_global (
		id, feature, key, value, description, secret, created
	) VALUES (
		'%s', 'feature3', 'my.key', '"stringval"', '%s', true, '%s'
	)`

		execQuery(ctx, t, pool, fmt.Sprintf(q, id, description, created.Format(time.RFC3339)))

		got, err := GetGlobalConfig(ctx, "feature3")
		if err != nil {
			t.Fatal(err)
		}

		want := []*Configuration{
			{
				ID:      id,
				Key:     "my.key",
				Content: []byte(`"stringval"`),
				Created: created,
				Source:  ConfigSourceGlobal,
			},
		}

		if !cmp.Equal(want, got) {
			t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got))
		}
	})
	t.Run("ConfigCreate_Environment", func(t *testing.T) {
		pool := newPool(ctx, t, container, dsn)
		ctx = setupContext(pool)

		envid := uuid.New()
		tenantid := uuid.New()
		q1 := `INSERT INTO tenants (id, name) VALUES ('%s', 'tenant1')`
		q2 := `INSERT INTO environments (id, tenant_id, name, kind) VALUES ('%s', '%s', 'env1', 'tenant')`

		execQuery(ctx, t, pool, fmt.Sprintf(q1, tenantid), fmt.Sprintf(q2, envid, tenantid))

		config := NewConfiguration{
			EnvironmentID: &envid,
			Feature:       "feature5",
			Description:   new("description"),
			Key:           "my.key",
			Value:         []byte(`"stringval"`),
			Secret:        true,
		}
		got, err := CreateEnvConfig(ctx, config)
		if err != nil {
			t.Fatal(err)
		}

		want := &Configuration{
			Key:     config.Key,
			Content: config.Value,
			Source:  ConfigSourceEnv,
		}

		opts := cmpopts.IgnoreFields(Configuration{}, "ID", "Created")
		if !cmp.Equal(want, got, opts) {
			t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got, opts))
		}
	})
	t.Run("ConfigCreate_Global", func(t *testing.T) {
		pool := newPool(ctx, t, container, dsn)
		ctx = setupContext(pool)

		config := NewConfiguration{
			Feature:     "feature5",
			Description: new("description"),
			Key:         "my.key",
			Value:       []byte(`"stringval"`),
			Secret:      true,
		}
		got, err := CreateGlobalConfig(ctx, config)
		if err != nil {
			t.Fatal(err)
		}

		want := &Configuration{
			Key:     config.Key,
			Content: config.Value,
			Source:  ConfigSourceGlobal,
		}

		opts := cmpopts.IgnoreFields(Configuration{}, "ID", "Created")
		if !cmp.Equal(want, got, opts) {
			t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got, opts))
		}
	})
	t.Run("UpdateConfig_Global", func(t *testing.T) {
		pool := newPool(ctx, t, container, dsn)
		ctx = setupContext(pool)

		config := NewConfiguration{
			Feature:     "feature5",
			Description: new("description"),
			Key:         "my.key",
			Value:       []byte(`"stringval"`),
			Secret:      true,
		}
		// Create
		got, err := CreateGlobalConfig(ctx, config)
		if err != nil {
			t.Fatal(err)
		}

		got, err = UpdateConfig(ctx, got.ID, UpdateConfiguration{
			Value: []byte(`"newval"`),
		})
		if err != nil {
			t.Fatal(err)
		}

		want := &Configuration{
			Key:     config.Key,
			Content: []byte(`"newval"`),
			Source:  ConfigSourceGlobal,
		}

		opts := cmpopts.IgnoreFields(Configuration{}, "ID", "Created")
		if !cmp.Equal(want, got, opts) {
			t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got, opts))
		}
	})
	t.Run("DeleteConfig", func(t *testing.T) {
		pool := newPool(ctx, t, container, dsn)
		ctx = setupContext(pool)

		config := NewConfiguration{
			Feature: "feature9",
			Key:     "my.key",
			Value:   []byte(`"stringval"`),
		}
		// Create
		got, err := CreateGlobalConfig(ctx, config)
		if err != nil {
			t.Fatal(err)
		}

		gotID := got.ID
		err = DeleteConfig(ctx, gotID)
		if err != nil {
			t.Fatal(err)
		}
		confs, err := GetGlobalConfig(ctx, config.Feature)
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
	})
	t.Run("HelmValues_Ok", func(t *testing.T) {
		pool := newPool(ctx, t, container, dsn)
		ctx = setupContext(pool)

		envid := uuid.New()
		tenantid := uuid.New()
		q1 := `INSERT INTO tenants (id, name) VALUES ('%s', 'tenant1')`
		q2 := `INSERT INTO environments (id, tenant_id, name, kind) VALUES ('%s', '%s', 'env1', 'tenant')`

		execQuery(ctx, t, pool, fmt.Sprintf(q1, tenantid), fmt.Sprintf(q2, envid, tenantid))

		feature := Feature{
			Name: "feature5",
			FeatureYAML: FeatureYAML{
				Values: Values{
					"my.key": Value{
						Config: &Config{
							Type:   ConfigTypeString,
							Secret: true,
						},
					},
				},
			},
		}

		config := NewConfiguration{
			EnvironmentID: &envid,
			Feature:       feature.Name,
			Key:           "my.key",
			Value:         []byte(`"stringval"`),
			Secret:        true,
		}
		// Create
		_, err := CreateEnvConfig(ctx, config)
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
	})
	t.Run("HelmValues_MissingRequiredFields", func(t *testing.T) {
		pool := newPool(ctx, t, container, dsn)
		ctx = setupContext(pool)

		envid := uuid.New()
		tenantid := uuid.New()
		q1 := `INSERT INTO tenants (id, name) VALUES ('%s', 'tenant1')`
		q2 := `INSERT INTO environments (id, tenant_id, name, kind) VALUES ('%s', '%s', 'env1', 'tenant')`
		execQuery(ctx, t, pool, fmt.Sprintf(q1, tenantid), fmt.Sprintf(q2, envid, tenantid))

		feature := Feature{
			Name: "feature5",
			FeatureYAML: FeatureYAML{
				Values: Values{
					"my.key": Value{
						Config: &Config{
							Type:   ConfigTypeString,
							Secret: true,
						},
					},
					"no.key": Value{
						Config: &Config{
							Type:   ConfigTypeString,
							Secret: true,
						},
						Required: true,
					},
				},
			},
		}

		config := NewConfiguration{
			EnvironmentID: &envid,
			Feature:       "feature5",
			Key:           "my.key",
			Value:         []byte(`"stringval"`),
			Secret:        true,
		}
		// Create
		_, err := CreateEnvConfig(ctx, config)
		if err != nil {
			t.Fatal(err)
		}

		_, err = HelmValues(ctx, &feature, envid)
		if !errors.Is(err, &errs.ErrMissingRequiredFields{}) {
			t.Errorf("got: %v, want ErrMissingRequiredFields", err)
		}
	})
	t.Run("HelmValues_InvalidKeyNesting", func(t *testing.T) {
		pool := newPool(ctx, t, container, dsn)
		ctx = setupContext(pool)

		envid := uuid.New()
		tenantid := uuid.New()
		q1 := `INSERT INTO tenants (id, name) VALUES ('%s', 'tenant1')`
		q2 := `INSERT INTO environments (id, tenant_id, name, kind) VALUES ('%s', '%s', 'env1', 'tenant')`
		execQuery(ctx, t, pool, fmt.Sprintf(q1, tenantid), fmt.Sprintf(q2, envid, tenantid))

		config := NewConfiguration{
			Feature: "feature5",
			Key:     "my.key",
			Value:   []byte(`"stringval"`),
			Secret:  true,
		}
		config2 := NewConfiguration{
			Feature: "feature5",
			Key:     "my",
			Value:   []byte(`15`),
			Secret:  true,
		}
		// Create
		_, err := CreateGlobalConfig(ctx, config2)
		if err != nil {
			t.Fatal(err)
		}
		_, err = CreateGlobalConfig(ctx, config)
		if err != nil {
			t.Fatal(err)
		}

		feature := &Feature{
			Name: "feature5",
			FeatureYAML: FeatureYAML{
				Values: Values{
					"my.key": Value{
						Config: &Config{
							Type: ConfigTypeString,
						},
					},
					"my": Value{
						Config: &Config{
							Type: ConfigTypeString,
						},
					},
				},
			},
		}
		_, err = HelmValues(ctx, feature, envid)
		if err == nil || !strings.HasSuffix(err.Error(), "is not nestable") {
			t.Errorf("got: %v, want \"key `key` is not nestable\"", err)
		}
	})
	t.Run("HelmValues_WithMappingValues", func(t *testing.T) {
		pool := newPool(ctx, t, container, dsn)
		ctx = setupContext(pool)

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

		f := Feature{
			Name: "feature5",
			FeatureYAML: FeatureYAML{
				Values: Values{
					"names.tenant": Value{
						Computed: &Computed{
							Template: "{{ .Tenant.Name }}",
						},
					},
					"names.environment": Value{
						Computed: &Computed{
							Template: "{{ .Env.name }}",
						},
					},
					"kind": Value{
						Computed: &Computed{
							Template: "{{ .Kind }}",
						},
					},
					"projects.env": Value{
						Computed: &Computed{
							Template: "{{ .Env.project_id }}",
						},
					},
					"projects.mgmt": Value{
						Computed: &Computed{
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
	})
	t.Run("HelmValues_WithIgnoredKeys_Ignored", func(t *testing.T) {
		pool := newPool(ctx, t, container, dsn)
		ctx = setupContext(pool)

		envid := uuid.New()
		tenantid := uuid.New()
		q1 := `INSERT INTO tenants (id, name) VALUES ('%s', 'tenant1')`
		q2 := `INSERT INTO environments (id, tenant_id, name, kind) VALUES ('%s', '%s', 'env1', 'onprem')`
		execQuery(ctx, t, pool, fmt.Sprintf(q1, tenantid), fmt.Sprintf(q2, envid, tenantid))

		feature := Feature{
			Name: "feature6",
			FeatureYAML: FeatureYAML{
				Values: Values{
					"my.key": Value{
						Config: &Config{
							Type:   ConfigTypeString,
							Secret: true,
						},
					},
					"ignore.key": Value{
						Required: true,
						IgnoreKind: []environment.EnvironmentKind{
							environment.EnvironmentKindOnprem,
						},
						Config: &Config{
							Type:   ConfigTypeString,
							Secret: true,
						},
					},
				},
			},
		}

		config := NewConfiguration{
			EnvironmentID: &envid,
			Feature:       feature.Name,
			Key:           "my.key",
			Value:         json.RawMessage(`"stringval"`),
			Secret:        true,
		}
		_, err := CreateEnvConfig(ctx, config)
		if err != nil {
			t.Fatal(err)
		}

		globConfig := NewConfiguration{
			Feature: feature.Name,
			Key:     "ignore.key",
			Value:   []byte(`"ignore"`),
			Secret:  true,
		}
		_, err = CreateGlobalConfig(ctx, globConfig)
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
	})
	t.Run("HelmValues_WithIgnoredKeys_NotIgnored", func(t *testing.T) {
		pool := newPool(ctx, t, container, dsn)
		ctx = setupContext(pool)

		envid := uuid.New()
		tenantid := uuid.New()
		q1 := `INSERT INTO tenants (id, name) VALUES ('%s', 'tenant1')`
		q2 := `INSERT INTO environments (id, tenant_id, name, kind) VALUES ('%s', '%s', 'env1', 'tenant')`
		execQuery(ctx, t, pool, fmt.Sprintf(q1, tenantid), fmt.Sprintf(q2, envid, tenantid))

		feature := Feature{
			Name: "feature6",
			FeatureYAML: FeatureYAML{
				Values: Values{
					"my.key": Value{
						Config: &Config{
							Type:   ConfigTypeString,
							Secret: true,
						},
					},
					"ignore.key": Value{
						Required: true,
						IgnoreKind: []environment.EnvironmentKind{
							environment.EnvironmentKindOnprem,
						},
						Config: &Config{
							Type:   ConfigTypeString,
							Secret: true,
						},
					},
				},
			},
		}

		config := NewConfiguration{
			EnvironmentID: &envid,
			Feature:       feature.Name,
			Key:           "my.key",
			Value:         []byte(`"stringval"`),
			Secret:        true,
		}
		_, err := CreateEnvConfig(ctx, config)
		if err != nil {
			t.Fatal(err)
		}

		globConfig := NewConfiguration{
			Feature: feature.Name,
			Key:     "ignore.key",
			Value:   []byte(`"ignore"`),
			Secret:  true,
		}
		_, err = CreateGlobalConfig(ctx, globConfig)
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
	})
}

func setupContext(pool *pgxpool.Pool) context.Context {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := Register(context.Background(), pool)
	ctx = audit.Register(ctx, pool, log)
	ctx = environment.Register(ctx, pool)
	return ctx
}

func startPostgresql(ctx context.Context, t *testing.T) (container *postgres.PostgresContainer, dsn string) {
	t.Helper()

	container, err := postgres.Run(
		ctx,
		"docker.io/postgres:16-alpine",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.WithSQLDriver("pgx"),
		postgres.BasicWaitStrategies(),
	)
	defer testcontainers.CleanupContainer(t, container)

	if err != nil {
		t.Fatal(err)
	}

	dsn, err = container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pool, _, err := database.NewConnPool(ctx, dsn, logger)
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()

	if err = container.Snapshot(ctx); err != nil {
		t.Fatal(err)
	}

	return container, dsn
}

func newPool(ctx context.Context, t *testing.T, container *postgres.PostgresContainer, dsn string) *pgxpool.Pool {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	pool, _, err := database.NewConnPool(ctx, dsn, log)
	if err != nil {
		t.Fatalf("Error connecting to database: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		if err = container.Restore(ctx); err != nil {
			t.Fatalf("failed to restore database: %v", err)
		}
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
