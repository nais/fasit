package message

import "encoding/json"

type ConsoleType string

const (
	ConsoleTypeCreateNamespace ConsoleType = "create-namespace"
	ConsoleTypeDeleteNamespace ConsoleType = "delete-namespace"
)

type Console struct {
	Type ConsoleType     `json:"type"`
	Data json.RawMessage `json:"data"`
}

type CreateNamespace struct {
	Name         string `json:"name"`
	GCPProject   string `json:"gcpProject"`
	GroupEmail   string `json:"groupEmail"`
	AzureGroupID string `json:"azureGroupID"`
	CNRMEmail    string `json:"cnrmEmail"`
}

type DeleteNamespace struct {
	Name string `json:"name"`
}
