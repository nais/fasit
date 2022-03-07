package status

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
	// Name is the name of the feature
	Name string
	// Version is the chart version
	Version       string
	RolloutStatus string
	ConfigHash    string
}
