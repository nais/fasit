package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type RolloutSummary struct {
	ID           uuid.UUID     `json:"id"`
	FeatureName  string        `json:"feature"`
	Status       RolloutStatus `json:"status"`
	Created      time.Time     `json:"created"`
	LastModified time.Time     `json:"lastModified"`
}

type Rollout struct {
	ID               uuid.UUID
	RolloutSummaryID uuid.UUID
	EnvironmentKind  EnvironmentKind
	Feature          string            `json:"-"`
	Status           RolloutStatus     `json:"status"`
	Changeset        *RolloutChangeset `json:"changeSet"`
	Created          time.Time         `json:"created"`
	LastModified     time.Time         `json:"lastModified"`
}

// RolloutChangeset contains new and old data in a one level map.
type RolloutChangeset struct {
	New map[string]json.RawMessage `json:"new"`
	Old map[string]json.RawMessage `json:"old"`
}

type RolloutEvent struct {
	ID        uuid.UUID
	RolloutID uuid.UUID
	Type      RolloutEventType
	Data      json.RawMessage
	Created   time.Time
}
