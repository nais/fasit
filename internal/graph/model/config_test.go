package model

import (
	"testing"
)

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
