package model

import (
	"time"

	"github.com/google/uuid"
)

type ClusterUpgradeStatus struct {
	ID            uuid.UUID     `json:"id"`
	UpgradeStatus UpgradeStatus `json:"upgradeStatus"`
	Version       string        `json:"version"`
	LastModified  time.Time     `json:"lastModified"`
	StartTime     time.Time     `json:"startTime"`
	EnvironmentID uuid.UUID     `json:"-"`
}

func (ClusterUpgradeStatus) IsUpdate() {}
