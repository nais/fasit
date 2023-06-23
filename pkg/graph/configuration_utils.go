package graph

import (
	"fmt"

	feature "github.com/nais/fasit/pkg/feature2"
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
			fmt.Print(f.Values[c.Value.GraphQLKey].IgnoreKind, " ")
			fmt.Println("ignoring", c.Value.GraphQLKey, "because it is ignored for", envKind)
			continue
		}
		ret = append(ret, c)
	}
	return ret
}
