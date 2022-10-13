package graph

import (
	"fmt"

	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/feature/helmdefault"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/sirupsen/logrus"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	Repo            database.Repo
	Features        *feature.Manager
	Log             *logrus.Entry
	HelmChartValues *helmdefault.Cache
}

func (r *Resolver) resolveFeatureByName(name string) (*model.Feature, error) {
	f := r.Features.Get(name)
	if f == nil {
		return nil, fmt.Errorf("feature %v not found", name)
	}
	return marshalFeature(*f)
}
