package message

import (
	"time"

	"github.com/google/uuid"
)

type DeployInstruction struct {
	// Name is the name of the feature
	Name string
	// Version is the chart version
	Version    string
	Chart      string
	Repo       string
	ConfigHash string
	Timeout    time.Duration
	Values     map[string]any
	RolloutIDs []uuid.UUID
}
