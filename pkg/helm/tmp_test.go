package helm

import (
	"fmt"
	"testing"
)

func TestSomething(t *testing.T) {
	b, err := DownloadChartFile("oci://europe-north1-docker.pkg.dev/nais-io/nais/clamav/clamav", "0.1.0-feature", "", "Feature.yaml")

	fmt.Println("err: ", err)
	fmt.Println(string(b))
}
