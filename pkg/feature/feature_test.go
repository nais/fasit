package feature

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/nais/fasit"
)

func TestFeature(t *testing.T) {
	mgr, err := New(fasit.FeaturesFS)
	if err != nil {
		t.Fatal(err)
	}

	spew.Dump(mgr.Features)
}
