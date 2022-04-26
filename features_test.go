package fasit

import (
	"testing"

	"github.com/nais/fasit/pkg/feature"
	"github.com/stevenle/topsort"
)

func TestFeatures_CircularDependency(t *testing.T) {
	mgr, err := feature.New(FeaturesFS)
	if err != nil {
		t.Fatal(err)
	}

	g := topsort.NewGraph()
	for _, f := range mgr.Features {
		for _, dep := range f.DependsOn {
			g.AddEdge(f.Name, dep)
		}
	}

	for _, f := range mgr.Features {
		if !g.ContainsNode(f.Name) {
			continue
		}
		_, err := g.TopSort(f.Name)
		if err != nil {
			t.Errorf("Feature %s has a circular dependency: %v", f.Name, err)
		}
	}
}
