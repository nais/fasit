package model

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Status struct {
	EnvironmentID uuid.UUID     `json:"environmentID"`
	Feature       string        `json:"feature"`
	Version       string        `json:"version"`
	Status        RolloutStatus `json:"status"`
	ConfigHash    string        `json:"configHash"`
	Created       time.Time     `json:"created"`
	LastModified  time.Time     `json:"lastModified"`
	Log           string        `json:"log"`

	DeployInstructionID uuid.UUID `json:"-"`
}

func (s *Status) IsUpdate() {}

type RolloutStatus string

const (
	RolloutStatusUnknown  RolloutStatus = ""
	RolloutStatusCreated  RolloutStatus = "created"
	RolloutStatusPending  RolloutStatus = "pending"
	RolloutStatusDeployed RolloutStatus = "deployed"
	RolloutStatusFailed   RolloutStatus = "failed"
)

var AllRolloutStatus = []RolloutStatus{
	RolloutStatusUnknown,
	RolloutStatusCreated,
	RolloutStatusPending,
	RolloutStatusDeployed,
	RolloutStatusFailed,
}

func (r RolloutStatus) IsValid() bool {
	switch r {
	case RolloutStatusUnknown, RolloutStatusCreated, RolloutStatusPending, RolloutStatusDeployed, RolloutStatusFailed:
		return true
	}
	return false
}

func (r RolloutStatus) String() string {
	return string(r)
}

func (r *RolloutStatus) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	str = strings.ToLower(str)

	if str == "unknown" {
		str = ""
	}

	*r = RolloutStatus(str)
	if !r.IsValid() {
		return fmt.Errorf("%s is not a valid RolloutStatus", str)
	}
	return nil
}

func (r RolloutStatus) MarshalGQL(w io.Writer) {
	str := r.String()
	if str == "" {
		str = "unknown"
	}
	fmt.Fprint(w, strconv.Quote(strings.ToUpper(str)))
}

type LogLine struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`

	IntID               int       `json:"-"`
	DeployInstructionID uuid.UUID `json:"-"`
}
