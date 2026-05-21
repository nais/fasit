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
)

// ObjectType represents the type of resource affected.
type ObjectType string

const (
	ObjectTypeDeployment       ObjectType = "deployment"
	ObjectTypeEnvironment      ObjectType = "environment"
	ObjectTypeEnvironmentValue ObjectType = "environment_value"
	ObjectTypeTenant           ObjectType = "tenant"
	ObjectTypeFeature          ObjectType = "feature"
	ObjectTypeConfiguration    ObjectType = "configuration"
)

type CreateParams struct {
	Action        Action
	Description   string
	ObjectType    ObjectType
	ObjectID      string
	EnvironmentID *uuid.UUID
	Metadata      any
}

type Entry struct {
	Actor         string
	Action        Action
	Description   string
	ObjectType    ObjectType
	ObjectID      string
	EnvironmentID *uuid.UUID
	CreatedAt     time.Time
	Metadata      json.RawMessage
}
