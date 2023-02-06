package helm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"path/filepath"
	"strings"

	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	helmRepo "helm.sh/helm/v3/pkg/repo"
)

func DownloadChart(chart, version, repo string) (*bytes.Buffer, error) {
	sb := &strings.Builder{}
	downloader := downloader.ChartDownloader{
		Out:    sb,
		Verify: downloader.VerifyNever,
	}

	settings := &cli.EnvSettings{}
	if repo != "" {
		chartURL, err := helmRepo.FindChartInAuthAndTLSAndPassRepoURL(repo, "", "", chart, version, "", "", "", false, false, getter.All(settings))
		if err != nil {
			return nil, err
		}
		chart = chartURL
	}

	u, err := downloader.ResolveChartVersion(chart, version)
	if err != nil {
		return nil, err
	}

	getters := getter.All(settings)
	get, err := getters.ByScheme(u.Scheme)
	if err != nil {
		return nil, err
	}

	return get.Get(u.String())
}
func DownloadChartFile(chart, version, repo, filename string) ([]byte, error) {
	resp, err := DownloadChart(chart, version, repo)
	if err != nil {
		return nil, err
	}

	var valuesYAML []byte
	gr, err := gzip.NewReader(resp)
	if err != nil {
		return nil, err
	}

	r := tar.NewReader(gr)
	for {
		hdr, err := r.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		if filepath.Base(hdr.Name) == filename {
			valuesYAML, err = io.ReadAll(r)
			if err != nil {
				return nil, err
			}
			break
		}
	}

	return valuesYAML, nil
}
