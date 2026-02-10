package deployment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/deployment/deploymentsql"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/graph/model"
)

type Deployment struct {
	ID          uuid.UUID      `json:"id"`
	Feature     *model.Feature `json:"feature"`
	Description *string        `json:"description"`
	Created     time.Time      `json:"created"`
	CI          bool           `json:"ci"`

	TargetLabels environment.Labels `json:"-"`
}

func (d *Deployment) Target() []*model.EnvironmentLabel {
	target := make([]*model.EnvironmentLabel, 0)
	for k, v := range d.TargetLabels {
		target = append(target, &model.EnvironmentLabel{
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

func getDeployment(ctx context.Context, querier deploymentsql.Querier, id uuid.UUID) (*Deployment, error) {
	d, err := querier.GetDeployment(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting deployment from db: %w", err)
	}

	ret, err := deploymentFromSQL(d.Deployment, d.FeatureDatum)
	if err != nil {
		return nil, fmt.Errorf("converting deployment from sql: %w", err)
	}

	return ret, nil
}

func makeFeatureYAML(fd deploymentsql.FeatureDatum) (model.FeatureYAML, map[string]json.RawMessage, error) {
	ret := model.FeatureYAML{
		Timeout: time.Duration(fd.Timeout) * time.Millisecond,
	}
	if err := json.Unmarshal(fd.Dependencies, &ret.Dependencies); err != nil {
		return ret, nil, fmt.Errorf("unmarshal dependencies: %w", err)
	}

	var retDefaultVals map[string]json.RawMessage
	if err := json.Unmarshal(fd.DefaultValues, &retDefaultVals); err != nil {
		return ret, nil, fmt.Errorf("unmarshal default values: %w", err)
	}

	ret.EnvironmentKinds = make([]model.EnvironmentKind, len(fd.Kinds))
	for i, k := range fd.Kinds {
		ret.EnvironmentKinds[i] = model.EnvironmentKind(k)
	}

	if err := json.Unmarshal(fd.Values, &ret.Values); err != nil {
		return ret, nil, fmt.Errorf("unmarshal values: %w", err)
	}

	if len(fd.Rename) > 0 {
		if err := json.Unmarshal(fd.Rename, &ret.Rename); err != nil {
			return ret, nil, fmt.Errorf("unmarshal rename: %w", err)
		}
	}

	return ret, retDefaultVals, nil
}

func featureFromSQL(f deploymentsql.FeatureDatum) (*model.Feature, error) {
	fyaml, defaultValues, err := makeFeatureYAML(f)
	if err != nil {
		return nil, fmt.Errorf("make feature yaml: %w", err)
	}

	return &model.Feature{
		FeatureYAML: fyaml,
		Name:        f.Name,
		Chart:       f.Chart,
		Version:     f.Version,
		Description: f.Description,
		Source:      f.Source,
		ValuesYAML:  defaultValues,
		SpecVersion: "v2",
	}, nil
}

func deploymentFromSQL(d deploymentsql.Deployment, fd deploymentsql.FeatureDatum) (*Deployment, error) {
	feature, err := featureFromSQL(fd)
	if err != nil {
		return nil, fmt.Errorf("make feature: %w", err)
	}
	feature.HasDeployments = true

	return &Deployment{
		ID:           d.ID,
		Feature:      feature,
		Description:  d.Description,
		Created:      d.Created.Time,
		CI:           d.Ci,
		TargetLabels: environment.Labels(d.Target),
	}, nil
}

func deployInstructionFromSQL(di deploymentsql.DeployInstruction) *model.DeployInstruction {
	return &model.DeployInstruction{
		ID:             di.ID,
		EnvironmentID:  di.EnvironmentID,
		DeploymentID:   di.DeploymentID,
		FeatureName:    di.FeatureName,
		FeatureVersion: di.FeatureVersion,
		Status:         model.RolloutStatus(di.Status),
		Hash:           di.Hash,
		Created:        di.Created.Time,
		LastModified:   di.LastModified.Time,
		Values:         di.Values,
	}
}
