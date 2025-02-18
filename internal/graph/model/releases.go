package model

import (
	"time"

	"github.com/google/uuid"
)

type Release struct {
	Name         string    `json:"name"`
	Version      string    `json:"version"`
	Status       string    `json:"status"`
	Revision     int       `json:"revision"`
	LastDeployed time.Time `json:"lastDeployed"`
	Created      time.Time `json:"created"`
	LastModified time.Time `json:"lastModified"`

	GraphVars struct {
		EnvironmentID uuid.UUID
	} `json:"-"`
}
