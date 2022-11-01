package model

import "encoding/json"

type Feature struct {
	Name             string            `json:"name"`
	Chart            string            `json:"chart"`
	Version          string            `json:"version"`
	Repo             string            `json:"repo"`
	Source           string            `json:"source"`
	DependsOn        []*Dependency     `json:"dependsOn"`
	Config           json.RawMessage   `json:"config"`
	EnvironmentKinds []EnvironmentKind `json:"environmentKinds"`
}
