package deployment

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/deployment/deploymentsql"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/graph/model"
	commonmodel "github.com/nais/fasit/internal/model"
)

type EnvironmentFeature struct {
	Name            string
	FeatureDisabled bool
}

type CreateDeployment struct {
	Chart       string
	Version     string
	Description *string
	Commit      *commonmodel.GitHubCommit
	Target      environment.Labels
}

type Deployment struct {
	ID          uuid.UUID      `json:"id"`
	Feature     *model.Feature `json:"feature"`
	Description *string        `json:"description"`
	GHRef       []byte         `json:"ghRef,omitempty"`
	Created     time.Time      `json:"created"`
	Active      bool           `json:"active"`

	TargetLabels    environment.Labels `json:"-"`
	TplDetails      []byte             `json:"-"`
	FeatureDisabled bool               `json:"-"`
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
	DeploymentStatusStateDisabled DeploymentStatusState = "DISABLED"
)

func (e DeploymentStatusState) IsValid() bool {
	switch e {
	case DeploymentStatusStateUnknown, DeploymentStatusStateCreated, DeploymentStatusStatePending, DeploymentStatusStateDeployed, DeploymentStatusStateFailed, DeploymentStatusStateDisabled:
		return true
	}
	return false
}

func (e DeploymentStatusState) String() string {
	return string(e)
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

	return &Deployment{
		ID:           d.ID,
		Feature:      feature,
		Description:  d.Description,
		GHRef:        d.GhRef,
		Created:      d.Created.Time,
		Active:       d.Active,
		TargetLabels: environment.Labels(d.Target),
		TplDetails:   fd.TplDetails,
	}, nil
}

func deploymentsFromRows(rows []deploymentsql.ListDeploymentsForEnvironmentRow) ([]*Deployment, error) {
	deps := make([]*Deployment, len(rows))
	for i, row := range rows {
		dep, err := deploymentFromSQL(row.Deployment, row.FeatureDatum)
		if err != nil {
			return nil, fmt.Errorf("make deployment: %w", err)
		}
		dep.FeatureDisabled = row.Disabled
		deps[i] = dep
	}
	return deps, nil
}
