package model

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/environment"
)

type Deployment struct {
	ID          uuid.UUID `json:"id"`
	Feature     *Feature  `json:"feature"`
	Description string    `json:"description"`
	Created     time.Time `json:"created"`

	TargetLabels environment.Labels `json:"-"`
}

func (d *Deployment) Target() []*EnvironmentLabel {
	target := make([]*EnvironmentLabel, 0)
	for k, v := range d.TargetLabels {
		target = append(target, &EnvironmentLabel{
			Key:   k,
			Value: v,
		})
	}
	return target
}

type DeploymentStatus struct {
	State        DeploymentStatusState `json:"state"`
	Message      string                `json:"message"`
	LastModified time.Time             `json:"lastModified"`
	Created      time.Time             `json:"created"`

	DeploymentID  uuid.UUID `json:"-"`
	EnvironmentID uuid.UUID `json:"-"`
}

type DeploymentStatusState string

const (
	DeploymentStatusStateUnknown  DeploymentStatusState = "UNKNOWN"
	DeploymentStatusStateCreated  DeploymentStatusState = "CREATED"
	DeploymentStatusStatePending  DeploymentStatusState = "PENDING"
	DeploymentStatusStateDeployed DeploymentStatusState = "DEPLOYED"
	DeploymentStatusStateFailed   DeploymentStatusState = "FAILED"
)

var AllDeploymentStatusState = []DeploymentStatusState{
	DeploymentStatusStateUnknown,
	DeploymentStatusStateCreated,
	DeploymentStatusStatePending,
	DeploymentStatusStateDeployed,
	DeploymentStatusStateFailed,
}

func (e DeploymentStatusState) IsValid() bool {
	switch e {
	case DeploymentStatusStateUnknown, DeploymentStatusStateCreated, DeploymentStatusStatePending, DeploymentStatusStateDeployed, DeploymentStatusStateFailed:
		return true
	}
	return false
}

func (e DeploymentStatusState) String() string {
	return string(e)
}

func (e *DeploymentStatusState) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = DeploymentStatusState(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid DeploymentStatusState", str)
	}
	return nil
}

func (e DeploymentStatusState) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

func (e *DeploymentStatusState) UnmarshalJSON(b []byte) error {
	s, err := strconv.Unquote(string(b))
	if err != nil {
		return err
	}
	return e.UnmarshalGQL(s)
}

func (e DeploymentStatusState) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	e.MarshalGQL(&buf)
	return buf.Bytes(), nil
}
