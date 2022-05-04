package model

import "time"

type Release struct {
	FeatureName  string
	Version      string    `json:"version"`
	Status       string    `json:"status"`
	Revision     int       `json:"revision"`
	LastDeployed time.Time `json:"lastDeployed"`
	Created      time.Time `json:"created"`
	LastModified time.Time `json:"lastModified"`
}
