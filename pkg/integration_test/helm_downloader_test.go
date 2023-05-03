package integration_test

import (
	"bytes"
	"net/url"
	"os"
	"path/filepath"

	"github.com/nais/fasit/pkg/graph/model"
)

func init() {
	model.DownloadChartFunc = func(chart string, version string, repo string) (*bytes.Buffer, error) {
		uri, err := url.Parse(chart)
		if err != nil {
			return nil, err
		}

		name := filepath.Base(uri.Host)

		b, err := os.ReadFile(filepath.Join("testdata", name+"-"+version+".tgz"))
		if err != nil {
			return nil, err
		}

		return bytes.NewBuffer(b), nil
	}
}
