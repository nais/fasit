package helmdefault

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nais/fasit/pkg/feature"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	helmRepo "helm.sh/helm/v3/pkg/repo"
)

type Cache struct {
	lock  *sync.Mutex
	cache map[string]chartutil.Values

	mgr *feature.Manager
	log logrus.FieldLogger
}

func New(mgr *feature.Manager, log logrus.FieldLogger) (*Cache, error) {
	return &Cache{
		lock: &sync.Mutex{},
		mgr:  mgr,
		log:  log,
	}, nil
}

func (c *Cache) Run(ctx context.Context, interval time.Duration) {
	c.log.Debug("starting helm chart value cache")

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		c.update(ctx)

		select {
		case <-ctx.Done():
			c.log.Debug("stopping helm chart value cache")
			return
		case <-ticker.C:
		}
	}
}

func (c *Cache) Get(feature string) chartutil.Values {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.cache[feature]
}

func (c *Cache) update(ctx context.Context) {
	newCache := make(map[string]chartutil.Values)

	for _, feature := range c.mgr.Features {
		log := c.log.WithField("feature", feature.Name)

		log.Debug("updating helm chart value cache")

		values, err := c.ValuesForChart(feature.Chart, feature.Version, feature.Repo)
		if err != nil {
			newCache[feature.Name] = c.Get(feature.Name)
			log.WithError(err).Error("failed to fetch values for chart")
			continue
		}

		newCache[feature.Name] = values
	}

	c.lock.Lock()
	c.cache = newCache
	c.lock.Unlock()
}

func (c *Cache) ValuesForChart(chart, version, repo string) (chartutil.Values, error) {
	sb := &strings.Builder{}
	d := downloader.ChartDownloader{
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

	u, err := d.ResolveChartVersion(chart, version)
	if err != nil {
		return nil, err
	}

	getters := getter.All(settings)
	get, err := getters.ByScheme(u.Scheme)
	if err != nil {
		return nil, err
	}

	resp, err := get.Get(u.String())
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
			return nil, err
		}

		if filepath.Base(hdr.Name) == "values.yaml" {
			valuesYAML, err = io.ReadAll(r)
			if err != nil {
				return nil, err
			}
			break
		}
	}

	return chartutil.ReadValues(valuesYAML)
}
