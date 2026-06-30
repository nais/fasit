package audit

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Action represents the type of mutation performed.
type Action string

const (
	ActionCreated   Action = "created"
	ActionUpdated   Action = "updated"
	ActionDeleted   Action = "deleted"
	ActionEnabled   Action = "enabled"
	ActionDisabled  Action = "disabled"
	ActionTriggered Action = "triggered"
	ActionRedeploy  Action = "redeploy"
	ActionUninstall Action = "uninstall"
)

// ObjectType represents the type of resource affected.
type ObjectType string

const (
	ObjectTypeFeatureAssignment ObjectType = "deployment"
	ObjectTypeEnvironment       ObjectType = "environment"
	ObjectTypeEnvironmentValue  ObjectType = "environment_value"
	ObjectTypeTenant            ObjectType = "tenant"
	ObjectTypeFeature           ObjectType = "feature"
	ObjectTypeConfiguration     ObjectType = "configuration"
)

type CreateParams struct {
	Action        Action
	Description   string
	ObjectType    ObjectType
	ObjectID      string
	Feature       string
	EnvironmentID *uuid.UUID
	Metadata      any
}

type Entry struct {
	Actor           string
	Action          Action
	Description     string
	ObjectType      ObjectType
	ObjectID        string
	EnvironmentID   *uuid.UUID
	EnvironmentName string
	TenantName      string
	CreatedAt       time.Time
	Metadata        json.RawMessage
}

// Summary returns a human-readable one-liner composed from the structured
// fields, e.g. "created deployment naiserator".
func (e *Entry) Summary() string {
	s := string(e.Action) + " " + e.ObjectType.Display() + " " + e.ObjectID
	if e.Description != "" {
		s += " (" + e.Description + ")"
	}
	return s
}

// Display returns a human-friendly label for the object type.
func (t ObjectType) Display() string {
	switch t {
	case ObjectTypeEnvironmentValue:
		return "env value"
	case ObjectTypeConfiguration:
		return "config"
	default:
		return string(t)
	}
}
