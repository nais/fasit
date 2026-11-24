// Package commandstatus defines the lifecycle statuses of a fasitd command,
// shared by the Fasit server and the fasitd agent.
package commandstatus

type Status string

const (
	Sent          Status = "sent"
	Installing    Status = "installing"
	Deployed      Status = "deployed"
	Uninstalled   Status = "uninstalled"
	Failed        Status = "failed"
	Undeliverable Status = "undeliverable"
)
