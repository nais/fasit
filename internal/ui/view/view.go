package view

type TenantNav struct {
	Name string
}

type EnvironmentNav struct {
	Name string
}

type FeatureNav struct {
	Name         string
	Enabled      bool
	FailedCount  int
	PendingCount int
}
