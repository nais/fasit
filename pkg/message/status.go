package message

type Status struct {
	Tenant      string
	Environment string
	Type        StatusType
	Data        []byte
}

type StatusType int

const (
	StatusTypeKubernetesEvent StatusType = iota + 1
	StatusTypeHelm
)

type Helm struct {
	// Name is the name of the feature
	Name string
	// Version is the chart version
	Version       string
	RolloutStatus string
	ConfigHash    string
	Log           string
}
