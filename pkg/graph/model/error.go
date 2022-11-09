package model

import "github.com/google/uuid"

type FeatureWarning struct {
	Message string `json:"message"`

	EnvironmentID uuid.UUID `json:"-"`
	FeatureName   string    `json:"-"`
}

func (FeatureWarning) IsWarning()           {}
func (f FeatureWarning) GetMessage() string { return f.Message }

type NaisdWarning struct {
	Message string `json:"message"`

	EnvironmentID uuid.UUID `json:"-"`
}

func (NaisdWarning) IsWarning()           {}
func (n NaisdWarning) GetMessage() string { return n.Message }
