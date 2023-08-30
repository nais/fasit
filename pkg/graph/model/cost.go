package model

import "github.com/google/uuid"

type CostSeries struct {
	Data     []float64 `json:"data"`
	TenantID uuid.UUID `json:"-"`
}

type EnvSeries struct {
	Data  []float64 `json:"data"`
	EnvID uuid.UUID `json:"-"`
}
