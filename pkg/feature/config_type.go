package feature

import (
	"encoding/json"
	"fmt"

	"github.com/nais/fasit/pkg/graph/model"
)

type ConfigType struct {
	Type        model.ConfigType `json:"type" yaml:"type" jsonschema:"enum=string,enum=int,enum=bool,enum=string_array"`
	Secret      bool             `json:"secret,omitempty" yaml:"secret,omitempty"`
	Required    bool             `json:"required,omitempty" yaml:"required,omitempty"`
	DisplayName string           `json:"displayName,omitempty" yaml:"displayName,omitempty"`
}

func (c ConfigType) Valid(value json.RawMessage) error {
	if c.Type == "" {
		return fmt.Errorf("type is invalid")
	}

	var v any
	if err := json.Unmarshal(value, &v); err != nil {
		return fmt.Errorf("unable to decode json: %w", err)
	}

	switch v := v.(type) {
	case string:
		if c.Type == model.ConfigTypeString {
			return nil
		}
	case int, int32, int64, float32, float64:
		if c.Type == model.ConfigTypeInt {
			return nil
		}
	case bool:
		if c.Type == model.ConfigTypeBool {
			return nil
		}
	case []any:
		if c.Type == model.ConfigTypeStringArray {
			if !isStringArray(v) {
				return fmt.Errorf("array contains non-string elements")
			}
			return nil
		}
	}
	if v == nil {
		return nil
	}
	return fmt.Errorf("value doesn't match the required type. Expected %v, got %T", c.Type, v)
}

func isStringArray(v []any) bool {
	for _, e := range v {
		if _, ok := e.(string); !ok {
			return false
		}
	}
	return true
}
