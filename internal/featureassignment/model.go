package featureassignment

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/featureassignment/featureassignmentsql"
	"github.com/nais/fasit/internal/graph/model"
	commonmodel "github.com/nais/fasit/internal/model"
)

type EnvironmentFeature struct {
	Name            string
	FeatureDisabled bool
}

type CreateFeatureAssignment struct {
	Chart       string
	Version     string
	Description *string
	Commit      *commonmodel.GitHubCommit
	Target      environment.Labels
}

type FeatureAssignment struct {
	ID          uuid.UUID      `json:"id"`
	Feature     *model.Feature `json:"feature"`
	Description *string        `json:"description"`
	GHRef       []byte         `json:"ghRef,omitempty"`
	Created     time.Time      `json:"created"`
	Active      bool           `json:"active"`

	TargetLabels    environment.Labels `json:"-"`
	FeatureDisabled bool               `json:"-"`
}

func (f *FeatureAssignment) Target() []*model.EnvironmentLabel {
	target := make([]*model.EnvironmentLabel, 0)
	for k, v := range f.TargetLabels {
		target = append(target, &model.EnvironmentLabel{
			Key:   k,
			Value: v,
		})
	}
	return target
}

type FeatureReconcileStatus struct {
	State        FeatureReconcileStatusState `json:"state"`
	Message      string                      `json:"message"`
	LastModified time.Time                   `json:"lastModified"`
	Created      time.Time                   `json:"created"`

	FeatureAssignmentID uuid.UUID `json:"-"`
	EnvironmentID       uuid.UUID `json:"-"`
}

type FeatureReconcileStatusState string

const (
	FeatureReconcileStatusUnknown       FeatureReconcileStatusState = "UNKNOWN"
	FeatureReconcileStatusStateCreated  FeatureReconcileStatusState = "CREATED"
	FeatureReconcileStatusStatePending  FeatureReconcileStatusState = "PENDING"
	FeatureReconcileStatusStateDeployed FeatureReconcileStatusState = "DEPLOYED"
	FeatureReconcileStatusStateFailed   FeatureReconcileStatusState = "FAILED"
	FeatureReconcileStatusStateDisabled FeatureReconcileStatusState = "DISABLED"
)

func makeFeatureYAML(fd featureassignmentsql.FeatureDatum) (model.FeatureYAML, map[string]json.RawMessage, error) {
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

func featureFromSQL(f featureassignmentsql.FeatureDatum) (*model.Feature, error) {
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
		TplDetails:  f.TplDetails,
	}, nil
}

func featureAssignmentFromSQL(d featureassignmentsql.FeatureAssignment, fd featureassignmentsql.FeatureDatum) (*FeatureAssignment, error) {
	feature, err := featureFromSQL(fd)
	if err != nil {
		return nil, fmt.Errorf("make feature: %w", err)
	}

	return &FeatureAssignment{
		ID:           d.ID,
		Feature:      feature,
		Description:  d.Description,
		GHRef:        d.GhRef,
		Created:      d.Created.Time,
		Active:       d.Active,
		TargetLabels: environment.Labels(d.Target),
	}, nil
}

func featureAssignmentsFromRows(rows []featureassignmentsql.ListFeatureAssignmentsForEnvironmentRow) ([]*FeatureAssignment, error) {
	deps := make([]*FeatureAssignment, len(rows))
	for i, row := range rows {
		dep, err := featureAssignmentFromSQL(row.FeatureAssignment, row.FeatureDatum)
		if err != nil {
			return nil, fmt.Errorf("make feature assignment: %w", err)
		}
		dep.FeatureDisabled = row.Disabled
		deps[i] = dep
	}
	return deps, nil
}
