package model

import (
	"time"

	"github.com/google/uuid"
)

type FeatureState struct {
	ID           string `json:"id"`
	FeatureName  string
	Enabled      bool       `json:"enabled"`
	EnabledAt    *time.Time `json:"enabledAt"`
	Created      time.Time  `json:"created"`
	LastModified time.Time  `json:"lastModified"`

	EnvID uuid.UUID `json:"-"`
}
