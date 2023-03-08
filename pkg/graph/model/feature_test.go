package model

import (
	"fmt"
	"testing"

	"github.com/davecgh/go-spew/spew"
)

func TestFeature(t *testing.T) {
	f, err := FromChart("oci://europe-north1-docker.pkg.dev/nais-io/nais/clamav/clamav", "0.1.0-feature")
	if err != nil {
		t.Fatal(err)
	}

	spew.Dump(f)
	fmt.Println(f.Chart)
}
