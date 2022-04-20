package database

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
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

	repo := Repo{querier: mq}
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
