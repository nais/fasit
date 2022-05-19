package model

import (
	"time"
)

type FeatureState struct {
	FeatureName  string
	Enabled      bool       `json:"enabled"`
	EnabledAt    *time.Time `json:"enabledAt"`
	Created      time.Time  `json:"created"`
	LastModified time.Time  `json:"lastModified"`
}
