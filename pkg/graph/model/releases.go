package model

import "time"

type Release struct {
	Name         string    `json:"name"`
	Version      string    `json:"version"`
	Status       string    `json:"status"`
	Revision     int       `json:"revision"`
	LastDeployed time.Time `json:"lastDeployed"`
	Created      time.Time `json:"created"`
	LastModified time.Time `json:"lastModified"`
}
