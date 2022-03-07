package model

import (
	"time"

	"github.com/google/uuid"
)

type Status struct {
	EnvironmentID uuid.UUID `json:"environmentID"`
	Feature       string    `json:"feature"`
	Version       string    `json:"version"`
	Status        string    `json:"status"`
	ConfigHash    string    `json:"configHash"`
	Created       time.Time `json:"created"`
	LastModified  time.Time `json:"lastModified"`
}
