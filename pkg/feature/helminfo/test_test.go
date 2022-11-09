package helminfo

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
)

func TestTest(t *testing.T) {
	// sb := &strings.Builder{}

	// regClient, err := registry.NewClient()
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// tags, err := d.RegistryClient.Tags("kyverno.github.io/kyverno")

	// d := downloader.ChartDownloader{
	// 	Out:            sb,
	// 	Verify:         downloader.VerifyNever,
	// 	RegistryClient: regClient,
	// }

	// tags, err := repo.FindChartInRepoURL("https://kyverno.github.io/kyverno", "kyverno", "", "", "", "", getter.All(&cli.EnvSettings{}))
	// if err != nil {
	// 	t.Fatal(err)
	// }

	// tags, err := d.RegistryClient.Tags("kyverno.github.io/kyverno")
	// if err != nil {
	// 	t.Fatal(err)
	// }

	// t.Log(tags)

	vers, err := chartVersion("kube-prometheus-stack", "41.7.3", "https://prometheus-community.github.io/helm-charts", 0)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("reloader", vers)

	vers, err = chartVersion("oci://europe-north1-docker.pkg.dev/nais-io/nais/monitoring", "1.10.0", "", 0)
	if err != nil {
		t.Fatal(err)
	}

	spew.Dump(vers.Outdated())
}
