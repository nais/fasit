//go:build integration_test

package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
)

func TestRepoEnvConfig(t *testing.T) {
	mq := &MockQuerier{
		envConfig: func(ctx context.Context, arg gensql.EnvConfigParams) ([]gensql.EnvConfigRow, error) {
			return []gensql.EnvConfigRow{
				{
					ID:      uuid.MustParse("00000000-0000-0000-0000-000000000001"),
					Feature: "feature",
					Secret:  false,
					Key:     "key",
					Value:   []byte("value"),
					Env:     true,
					Rank:    1,
				},
			}, nil
		},
	}

	expected := []*model.Configuration{
		{
			ID:      uuid.MustParse("00000000-0000-0000-0000-000000000001"),
			Feature: "feature",
			Key:     "key",
			Value:   []byte("value"),
			Env:     true,
		},
	}

	repo := repo{querier: mq}
	ec, err := repo.EnvConfig(context.Background(), "feature", uuid.New())
	if err != nil {
		t.Fatal(err)
	}

	if !cmp.Equal(ec, expected) {
		t.Error(cmp.Diff(ec, expected))
	}
}

func TestHelmConfigMap(t *testing.T) {
	jsonify := func(v any) json.RawMessage {
		b, _ := json.Marshal(v)
		return b
	}
	tests := map[string]struct {
		input    []gensql.ConfigForEnvRow
		expected map[string]any
	}{
		"empty": {
			input:    nil,
			expected: make(map[string]any),
		},
		"single_level": {
			input: []gensql.ConfigForEnvRow{
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
			input: []gensql.ConfigForEnvRow{
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
	envid := uuid.Nil
	created := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	description := "description"
	q := `INSERT INTO configurations (
		id, environment_id, feature, key, value, description, secret, created
	) VALUES (
		'%s', '%s', 'feature3', 'my.key', '"stringval"', '%s', true, '%s'
	)`
	repo := newTestRepo(t, fmt.Sprintf(q, id, envid, description, created.Format(time.RFC3339)))
	defer repo.Close()

	got, err := repo.ConfigGet(context.Background(), "feature3")
	if err != nil {
		t.Fatal(err)
	}

	want := []*model.Configuration{
		{
			ID:            id,
			EnvironmentID: &envid,
			Feature:       "feature3",
			Key:           "my.key",
			Value:         []byte(`"stringval"`),
			Env:           false,
			Created:       created,
			Description:   &description,
			Secret:        true,
		},
	}

	if !cmp.Equal(want, got) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got))
	}
}

func TestRepo_ConfigGetForEnv(t *testing.T) {
	id := uuid.New()
	envid := uuid.New()
	created := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	description := "description"
	q := `INSERT INTO configurations (
		id, environment_id, feature, key, value, description, secret, created
	) VALUES (
		'%s', '%s', 'feature3', 'my.key', '"stringval"', '%s', true, '%s'
	)`

	repo := newTestRepo(t,
		fmt.Sprintf(q, id, envid, description, created.Format(time.RFC3339)),
	)
	defer repo.Close()

	got, err := repo.ConfigGetForEnv(context.Background(), "feature3", envid)
	if err != nil {
		t.Fatal(err)
	}

	want := []*model.Configuration{
		{
			ID:            id,
			EnvironmentID: &envid,
			Feature:       "feature3",
			Key:           "my.key",
			Value:         []byte(`"stringval"`),
			Env:           true,
			Created:       created,
			Description:   &description,
			Secret:        true,
		},
	}

	if !cmp.Equal(want, got) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got))
	}
}

func TestRepo_ConfigCreate_Environment(t *testing.T) {
	repo := newTestRepo(t)
	defer repo.Close()
	envid := uuid.New()
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

	want := &model.Configuration{
		EnvironmentID: config.EnvironmentID,
		Feature:       config.Feature,
		Key:           config.Key,
		Description:   config.Description,
		Value:         config.Value,
		Secret:        config.Secret,
		Env:           true,
	}

	opts := cmpopts.IgnoreFields(model.Configuration{}, "ID", "Created")
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

	want := &model.Configuration{
		EnvironmentID: config.EnvironmentID,
		Feature:       config.Feature,
		Key:           config.Key,
		Description:   config.Description,
		Value:         config.Value,
		Secret:        config.Secret,
		Env:           false,
	}

	opts := cmpopts.IgnoreFields(model.Configuration{}, "ID", "Created")
	if !cmp.Equal(want, got, opts) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got, opts))
	}
}

func TestRepo_ConfigUpdate_Environment(t *testing.T) {
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
	// Create
	got, err := repo.ConfigCreate(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}

	got, err = repo.ConfigUpdate(context.Background(), got.ID, model.UpdateConfiguration{
		Value: []byte(`"newval"`),
	})

	want := &model.Configuration{
		EnvironmentID: config.EnvironmentID,
		Feature:       config.Feature,
		Key:           config.Key,
		Description:   nil,
		Value:         []byte(`"newval"`),
		Secret:        config.Secret,
		Env:           false,
	}

	opts := cmpopts.IgnoreFields(model.Configuration{}, "ID", "Created")
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

	err = r.ConfigDelete(context.Background(), got.ID)
	if err != nil {
		t.Fatal(err)
	}

	err = r.(*repo).db.QueryRow(`SELECT id FROM configurations WHERE id = $1`, got.ID).Scan()
	if err != sql.ErrNoRows {
		t.Errorf("got: %v, want %v", err, sql.ErrNoRows)
	}
}

func TestRepo_HelmValues_OK(t *testing.T) {
	r := newTestRepo(t)
	defer r.Close()

	envid := uuid.New()
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

	got, err := r.HelmValues(context.Background(), "feature5", envid, []string{"my.key"})
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]any{
		"my": map[string]any{
			"key": json.RawMessage(`"stringval"`),
		},
	}

	if !cmp.Equal(want, got) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got))
	}
}

func TestRepo_HelmValues_MissingRequiredField(t *testing.T) {
	r := newTestRepo(t)
	defer r.Close()

	envid := uuid.New()
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

	_, err = r.HelmValues(context.Background(), "feature5", envid, []string{"no.key"})
	if !errors.Is(err, &ErrMissingRequiredFields{}) {
		t.Errorf("got: %v, want ErrMissingRequiredFields", err)
	}
}

func TestRepo_HelmValues_InvaldKeyNesting(t *testing.T) {
	r := newTestRepo(t)
	defer r.Close()

	envid := uuid.New()
	config := model.NewConfiguration{
		EnvironmentID: &envid,
		Feature:       "feature5",
		Key:           "my.key",
		Value:         []byte(`"stringval"`),
		Secret:        true,
	}
	config2 := model.NewConfiguration{
		EnvironmentID: &envid,
		Feature:       "feature5",
		Key:           "my",
		Value:         []byte(`15`),
		Secret:        true,
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

	_, err = r.HelmValues(context.Background(), "feature5", envid, nil)
	if err == nil || !strings.HasSuffix(err.Error(), "is not nestable") {
		t.Errorf("got: %v, want \"key `key` is not nestable\"", err)
	}
}
