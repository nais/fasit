package deployment

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/deployment/deploymentsql"
	"github.com/nais/fasit/internal/graph/model"
)

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

func deploymentFromSQL(d deploymentsql.Deployment, fd deploymentsql.FeatureDatum) (*model.Deployment, error) {
	feature, err := featureFromSQL(fd)
	if err != nil {
		return nil, fmt.Errorf("make feature: %w", err)
	}
	feature.HasDeployments = true

	return &model.Deployment{
		ID:           d.ID,
		Feature:      feature,
		Description:  d.Description,
		Created:      d.Created.Time,
		CI:           d.Ci,
		TargetLabels: d.Target,
	}, nil
}

func getDeployment(ctx context.Context, querier deploymentsql.Querier, id uuid.UUID) (*model.Deployment, error) {
	d, err := querier.DeploymentGet(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting deployment from db: %w", err)
	}

	ret, err := deploymentFromSQL(d.Deployment, d.FeatureDatum)
	if err != nil {
		return nil, fmt.Errorf("converting deployment from sql: %w", err)
	}

	return ret, nil
}
