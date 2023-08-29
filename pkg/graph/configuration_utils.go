package graph

import (
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph/model"
)

func removeIgnoredKinds(old []*model.Configuration, f *model.Feature, envKind model.EnvironmentKind) (ret []*model.Configuration) {
	for _, c := range old {
		if feature.ContainsKind(f.Values[c.Key].IgnoreKind, envKind) {
			continue
		}
		ret = append(ret, c)
	}
	return ret
}

func removeComputedIgnoredKinds(old []*model.ComputedValue, f *model.Feature, envKind model.EnvironmentKind) (ret []*model.ComputedValue) {
	for _, c := range old {
		if feature.ContainsKind(f.Values[c.Value.GraphQLKey].IgnoreKind, envKind) {
			continue
		}
		ret = append(ret, c)
	}
	return ret
}
