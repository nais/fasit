package model

import "github.com/google/uuid"

type ConfigOverride struct {
	EnvironmentID uuid.UUID `json:"-"`
	Keys          []string  `json:"keys"`
}
