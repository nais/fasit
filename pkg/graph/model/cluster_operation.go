package model

import (
	"time"

	"github.com/google/uuid"
)

type ClusterOperation struct {
	ID                  uuid.UUID
	OperationName       string
	TenantID            uuid.UUID
	EnvironmentID       uuid.UUID
	UpgradeID           uuid.UUID
	Status              string
	Type                string
	Detail              string
	NodesTotal          int
	NodesFailed         int
	NodesCompleted      int
	NodesDone           int
	NodePdbDelaySeconds int
	StartTime           time.Time
	LastModified        time.Time
}
