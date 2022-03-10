package model

import (
	"encoding/json"
)

type NewConfiguration struct {
	EnvironmentID *ID             `json:"environmentID"`
	Feature       string          `json:"feature"`
	Description   *string         `json:"description"`
	Key           string          `json:"key"`
	Value         json.RawMessage `json:"value"`
	Secret        bool
}
