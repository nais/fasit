package database

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/c3po/pkg/database/gensql"
)

func TestHelmConfigMap(t *testing.T) {
	jsonify := func(v interface{}) json.RawMessage {
		b, _ := json.Marshal(v)
		return b
	}
	tests := map[string]struct{
		input []gensql.ConfigForEnvRow
		expected map[string]interface{}
	}{
		"empty": {
			input:    nil,
			expected: make(map[string]interface{}),
		},
		"single_level": {
			input:    []gensql.ConfigForEnvRow{
				{
					Key:           "test1",
					Value:         jsonify("value1"),
				},
				{
					Key:           "test2",
					Value:         jsonify("value2"),
				},
			},
			expected: map[string]interface{}{
				"test1": jsonify("value1"),
				"test2": jsonify("value2"),
			},
		},
		"multi_level": {
			input:    []gensql.ConfigForEnvRow{
				{
					Key:           "test.a",
					Value:         jsonify("value_a"),
				},
				{
					Key:           "test.b",
					Value:         jsonify("value_b"),
				},
			},
			expected: map[string]interface{}{
				"test": map[string]interface{}{
					"a": jsonify("value_a"),
					"b": jsonify("value_b"),
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run( name, func(t *testing.T) {
			retval, err := makeHelmConfigMap(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			if !cmp.Equal(retval, tc.expected) {
				t.Error(cmp.Diff(retval, tc.expected))
			}
		} )
	}
}
