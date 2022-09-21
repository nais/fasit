package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Rollout struct {
	ID           uuid.UUID
	Feature      string            `json:"-"`
	Status       RolloutStatus     `json:"status"`
	Changeset    *RolloutChangeset `json:"changeSet"`
	Created      time.Time         `json:"created"`
	LastModified time.Time         `json:"lastModified"`
}

// RolloutChangeset contains new and old data in a one level map.
type RolloutChangeset struct {
	New map[string]json.RawMessage `json:"new"`
	Old map[string]json.RawMessage `json:"old"`
}
