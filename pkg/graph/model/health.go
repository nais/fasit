package model

import (
	"time"

	"github.com/google/uuid"
)

type Health struct {
	EnvironmentID uuid.UUID `json:"environmentID"`
	ReportedAt    time.Time `json:"reportedAt"`
}
