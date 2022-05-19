package fasit

import (
	"encoding/json"
	"testing"

	"github.com/nais/fasit/pkg/feature"
	"github.com/stevenle/topsort"
)

func TestFeatures(t *testing.T) {
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
		t.Run(f.Name, func(t *testing.T) {
			for name, cfg := range f.Config {
				if cfg.Default != nil && cfg.Valid(json.RawMessage(cfg.Default)) != nil {
					t.Errorf("config %s has invalid default value: %v", name, cfg.Valid(json.RawMessage(cfg.Default)))
				}
			}
			if len(f.EnvironmentKinds) == 0 {
				t.Error("no environment kind specified")
			}

			for _, kind := range f.EnvironmentKinds {
				if !kind.IsValid() {
					t.Errorf("invalid environment kind: %s", kind)
				}
			}

			if !g.ContainsNode(f.Name) {
				return
			}
			_, err := g.TopSort(f.Name)
			if err != nil {
				t.Errorf("Feature %s has a circular dependency: %v", f.Name, err)
			}
		})
	}
}
