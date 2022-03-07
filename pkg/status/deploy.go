package status

type DeployInstruction struct {
	// Name is the name of the feature
	Name string
	// Version is the chart version
	Version    string
	Chart      string
	Repo       string
	ConfigHash string
}
