package status

type Update struct {
	Partner     string
	Environment string
	Type        UpdateType
	Data        []byte
}

type UpdateType int

const (
	UpdateTypeKubernetesEvent UpdateType = iota + 1
	UpdateTypeHelm
)

type Helm struct {
	// Name is the name of the feature
	Name string
	// Version is the chart version
	Version       string
	RolloutStatus string
	ConfigHash    string
}
