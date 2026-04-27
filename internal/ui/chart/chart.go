package chart

import (
	"archive/tar"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

func FetchFeatureYAML(chartRef, version string) (string, error) {
	ref := strings.TrimPrefix(chartRef, "oci://") + ":" + version

	img, err := crane.Pull(ref)
	if err != nil {
		return "", fmt.Errorf("pull chart: %w", err)
	}

	layers, err := img.Layers()
	if err != nil {
		return "", fmt.Errorf("get layers: %w", err)
	}

	for _, layer := range layers {
		content, err := extractFeatureYAML(layer)
		if err == nil {
			return content, nil
		}
	}

	return "", fmt.Errorf("Feature.yaml not found in chart %s:%s", chartRef, version)
}

func extractFeatureYAML(layer v1.Layer) (string, error) {
	rc, err := layer.Uncompressed()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if path.Base(hdr.Name) == "Feature.yaml" {
			data, err := io.ReadAll(tr)
			if err != nil {
				return "", err
			}
			return string(data), nil
		}
	}

	return "", fmt.Errorf("not found")
}
