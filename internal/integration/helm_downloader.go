package integration

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"

	"github.com/nais/fasit/internal/graph/model"
)

var RootDir = ""

func init() {
	model.DownloadChartFunc = func(chart string, version string, repo string) (*bytes.Buffer, error) {
		uri, err := url.Parse(chart)
		if err != nil {
			return nil, err
		}

		name := filepath.Base(uri.Host)

		r, nerr := dirChart(name, version)
		if nerr != nil {
			return nil, fmt.Errorf("origErr: %q, dirChart: %q", err, nerr)
		}
		return r, nil
	}
}

func dirChart(name string, version string) (*bytes.Buffer, error) {
	root, err := os.OpenRoot(filepath.Join(RootDir, "_dir_charts"))
	if err != nil {
		return nil, err
	}

	filename := name + "-" + version
	stat, err := root.Stat(filename)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}
	if !stat.IsDir() {
		return nil, fmt.Errorf("not a directory")
	}

	absPath, err := root.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}

	var buf bytes.Buffer
	if err := tarGz(name, &buf, absPath.Name()); err != nil {
		return nil, err
	}

	return &buf, nil
}

func tarGz(name string, w io.Writer, path string) error {
	gw := gzip.NewWriter(w)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	return filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("walking path %q: %w", path, err)
		}

		if info.IsDir() {
			return nil
		}

		hdr, err := tar.FileInfoHeader(info, path)
		if err != nil {
			return fmt.Errorf("creating tar header: %w", err)
		}

		hdr.Name = filepath.Join(name, hdr.Name) // #nosec G305

		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("writing tar header: %w", err)
		}

		f, err := os.Open(path) // #nosec G304
		if err != nil {
			return fmt.Errorf("opening file %q: %w", path, err)
		}
		defer f.Close()

		if _, err := io.Copy(tw, f); err != nil {
			return fmt.Errorf("copying file %q: %w", path, err)
		}

		return nil
	})
}
