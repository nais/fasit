package model

import (
	"time"

	"github.com/google/uuid"
)

type ClusterUpgradeStatus struct {
	ID                    uuid.UUID     `json:"id"`
	UpgradeStatus         UpgradeStatus `json:"upgradeStatus"`
	Version               string        `json:"version"`
	LastModified          time.Time     `json:"lastModified"`
	StartTime             time.Time     `json:"startTime"`
	SlackMessageTimestamp string        `json:"slackMessageTimestamp"`
	SlackChannelID        string        `json:"slackChannelID"`
	IsAutomatic           *bool         `json:"isAutomatic"`

	EnvironmentID uuid.UUID `json:"-"`
}

func (ClusterUpgradeStatus) IsUpdate() {}
