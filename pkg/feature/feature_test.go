package feature

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/nais/c3po"
)

func TestFeature(t *testing.T) {
	mgr, err := New(c3po.FeaturesFS)
	if err != nil {
		t.Fatal(err)
	}

	spew.Dump(mgr.Features)
}
