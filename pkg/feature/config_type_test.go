package feature

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/nais/fasit/pkg/graph/model"
)

func TestConfigType_Valid(t *testing.T) {
	tests := map[string]struct {
		typ         model.ConfigType
		input       json.RawMessage
		expectedErr error
	}{
		"invalid type": {
			typ:         "",
			input:       json.RawMessage(`"invalid"`),
			expectedErr: fmt.Errorf("type is invalid"),
		},
		"invalid json": {
			typ:         model.ConfigTypeString,
			input:       json.RawMessage(`'invalid'`),
			expectedErr: fmt.Errorf("unable to decode json: invalid character '\\'' looking for beginning of value"),
		},

		"nil-bool": {
			typ:         model.ConfigTypeBool,
			input:       []byte("null"),
			expectedErr: nil,
		},
		"bool": {
			typ:         model.ConfigTypeBool,
			input:       []byte("true"),
			expectedErr: nil,
		},
		"invalid-bool": {
			typ:         model.ConfigTypeBool,
			input:       json.RawMessage(`"invalid"`),
			expectedErr: fmt.Errorf("value doesn't match the required type. Expected %v, got string", model.ConfigTypeBool),
		},

		"nil-int": {
			typ:         model.ConfigTypeInt,
			input:       []byte("null"),
			expectedErr: nil,
		},
		"int": {
			typ:         model.ConfigTypeInt,
			input:       []byte("13"),
			expectedErr: nil,
		},
		"invalid-int": {
			typ:         model.ConfigTypeInt,
			input:       json.RawMessage(`"invalid"`),
			expectedErr: fmt.Errorf("value doesn't match the required type. Expected %v, got string", model.ConfigTypeInt),
		},

		"nil-string": {
			typ:         model.ConfigTypeString,
			input:       []byte("null"),
			expectedErr: nil,
		},
		"string": {
			typ:         model.ConfigTypeString,
			input:       []byte(`"test"`),
			expectedErr: nil,
		},
		"invalid-string": {
			typ:         model.ConfigTypeString,
			input:       json.RawMessage(`1337`),
			expectedErr: fmt.Errorf("value doesn't match the required type. Expected %v, got float64", model.ConfigTypeString),
		},

		"nil-string-array": {
			typ:         model.ConfigTypeStringArray,
			input:       []byte("null"),
			expectedErr: nil,
		},
		"string-array": {
			typ:         model.ConfigTypeStringArray,
			input:       []byte(`["test"]`),
			expectedErr: nil,
		},
		"int-string-array": {
			typ:         model.ConfigTypeStringArray,
			input:       []byte(`["test", "test2", 13]`),
			expectedErr: fmt.Errorf("array contains non-string elements"),
		},
		"invalid-string-array": {
			typ:         model.ConfigTypeStringArray,
			input:       json.RawMessage(`"invalid"`),
			expectedErr: fmt.Errorf("value doesn't match the required type. Expected %v, got string", model.ConfigTypeStringArray),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ct := ConfigType{
				Type: tc.typ,
			}

			err := ct.Valid(tc.input)
			if errMsg(err) != errMsg(tc.expectedErr) {
				t.Errorf("expected %q, got %q", errMsg(tc.expectedErr), errMsg(err))
			}
		})
	}
}

func errMsg(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
