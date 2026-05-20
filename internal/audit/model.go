package audit

import (
	"encoding/json"
	"time"
)

type CreateParams struct {
	Description string
	ObjectType  string
	ObjectID    string
	Metadata    any
}

type Entry struct {
	Actor       string
	Description string
	ObjectType  string
	ObjectID    string
	CreatedAt   time.Time
	Metadata    json.RawMessage
}
