package model

import (
	"github.com/google/uuid"
)

type HelmValueDiff struct {
	Difference HelmValueDifference `json:"difference"`
	Diff       string              `json:"diff"`
}

type RolloutLog struct {
	ID          uuid.UUID  `json:"id"`
	TenantName  string     `json:"tenantName"`
	Environment string     `json:"environment"`
	Lines       []*LogLine `json:"lines"`
}

type HelmValueDifference string

const (
	HelmValueDifferenceFullMatch     HelmValueDifference = "FULL_MATCH"
	HelmValueDifferenceSupersetMatch HelmValueDifference = "SUPERSET_MATCH"
	HelmValueDifferenceNoMatch       HelmValueDifference = "NO_MATCH"
	HelmValueDifferenceInvalidJSON   HelmValueDifference = "INVALID_JSON"
)
