package model

import (
	"encoding/json"

	"github.com/google/uuid"
)

type EnvironmentValue struct {
	EnvironmentID uuid.UUID

	Key       string          `json:"key"`
	Value     json.RawMessage `json:"value"`
	Secret    bool            `json:"secret"`
	KnownUses int             `json:"knownUses"`
}
