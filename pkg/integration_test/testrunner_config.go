package integration

import "github.com/nais/fasit/pkg/graph/model"

type NAISD struct {
	// Enabled is a flag that indicates whether naisd should be enabled
	Enabled bool `json:"enabled"`
	// SuccessfullMessages is the number of successfull messages before naisd starts to fail
	SuccessfullMessages int `yaml:"successfullMessages,omitempty" json:"successfullMessages,omitempty"`
}

type Env struct {
	// Environment name
	Name string `json:"name"`
	// Reconcile is a flag that indicates whether the environment should be reconciled
	Reconcile bool `json:"reconcile,omitempty"`
	// Naisd is a flag that indicates whether the environment should be reconciled by naisd
	NAISD NAISD `json:"naisd,omitempty"`
	// CI is a flag that indicates whether the environment should be targeted by rollouts
	CI bool `json:"ci,omitempty"`
	// Kind is the environment kind
	Kind model.EnvironmentKind `json:"kind" jsonschema:"enum=management,enum=tenant,enum=onprem,enum=legacy"`
}

type Tenant struct {
	// Tenant name
	Name string `json:"name"`
	// Envs is a list of environments for the tenant
	Envs []Env `json:"envs"`
	// CI is a flag that indicates whether the tenant should be targeted by rollouts
	CI bool `json:"ci,omitempty"`
}

type Config struct {
	// List of tenants
	Tenants []Tenant `json:"tenants,omitempty"`
}
