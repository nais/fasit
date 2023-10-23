package message

import (
	"time"

	"github.com/google/uuid"
)

type DeployInstruction struct {
	ID uuid.UUID `json:",omitempty"`
	// Name is the name of the feature
	Name string
	// Version is the chart version
	Version    string
	Chart      string
	ConfigHash string
	Timeout    time.Duration
	Values     map[string]any
	Uninstall  bool `json:",omitempty"`
}
