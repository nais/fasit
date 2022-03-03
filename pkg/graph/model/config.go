package model

import (
	"encoding/json"

	"github.com/google/uuid"
)

type NewConfiguration struct {
	EnvironmentID *uuid.UUID      `json:"environmentID"`
	Feature       string          `json:"feature"`
	Description   *string         `json:"description"`
	Key           string          `json:"key"`
	Value         json.RawMessage `json:"value"`
	Secret        bool
}
