package workers

type StatusUpdate struct {
	Partner     string
	Environment string
	Type        StatusUpdateType
	Data        []byte
}

type StatusUpdateType int

const (
	StatusUpdateTypeKubernetesEvent StatusUpdateType = iota + 1
	StatusUpdateTypeHelm
)

type HelmStatus struct {
	Name          string
	Version		  string
	RolloutStatus string
}
