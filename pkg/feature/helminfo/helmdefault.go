package helminfo

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/nais/fasit/pkg/feature"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"
	helmRepo "helm.sh/helm/v3/pkg/repo"
)

type Chart struct {
	Values  chartutil.Values
	Version *ChartVersion
}

type Cache struct {
	lock  *sync.Mutex
	cache map[string]Chart

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

func (c *Cache) GetValues(feature string) chartutil.Values {
	c.lock.Lock()
	defer c.lock.Unlock()

	if v, ok := c.cache[feature]; ok {
		return v.Values
	}
	return nil
}

func (c *Cache) GetVersion(feature string) *ChartVersion {
	c.lock.Lock()
	defer c.lock.Unlock()

	if v, ok := c.cache[feature]; ok {
		return v.Version
	}
	return nil
}

func (c *Cache) AllVersions() map[string]*ChartVersion {
	c.lock.Lock()
	defer c.lock.Unlock()

	versions := make(map[string]*ChartVersion)

	for feature, chart := range c.cache {
		versions[feature] = chart.Version
	}

	return versions
}

func (c *Cache) get(feature string) Chart {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.cache[feature]
}

func (c *Cache) update(ctx context.Context) {
	newCache := make(map[string]Chart)

	for _, feature := range c.mgr.Features() {
		log := c.log.WithField("feature", feature.Name)

		// log.Debug("updating helm chart value cache")

		values, err := c.ValuesForChart(feature.Chart, feature.Version, feature.Repo)
		if err != nil {
			newCache[feature.Name] = c.get(feature.Name)
			log.WithError(err).WithField("feature", feature.Name).Error("failed to fetch values for chart")
			continue
		}

		version, err := chartVersion(feature.Chart, feature.Version, feature.Repo, 0)
		if err != nil {
			newCache[feature.Name] = c.get(feature.Name)
			log.WithError(err).Error("failed to fetch latest version for chart")
			continue
		}

		newCache[feature.Name] = Chart{Values: values, Version: version}
	}

	c.lock.Lock()
	c.cache = newCache
	c.lock.Unlock()
}

func (c *Cache) ValuesForChart(chart, version, repo string) (chartutil.Values, error) {
	valuesYAML, err := downloadChartFile(chart, version, repo, "values.yaml")
	if err != nil {
		return nil, err
	}

	return chartutil.ReadValues(valuesYAML)
}

func downloadChartFile(chart, version, repo, filename string) ([]byte, error) {
	resp, err := downloadChart(chart, version, repo)
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

func downloadChart(chart, version, repo string) (*bytes.Buffer, error) {
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

type ChartVersion struct {
	Name         string
	NewVersion   string `json:"version"`
	Current      string
	Dependencies []ChartVersion `json:"dependencies"`
}

func (c *ChartVersion) Outdated() bool {
	if c.Current == "" {
		return false
	}

	v, err := semver.NewVersion(c.NewVersion)
	if err != nil {
		return false
	}

	cv, err := semver.NewVersion(c.Current)
	if err != nil {
		return false
	}

	return v.GreaterThan(cv)
}

func chartVersion(chart, version, repoPath string, callNum int) (*ChartVersion, error) {
	if callNum > 10 {
		return nil, fmt.Errorf("too many redirects")
	}

	var ret *ChartVersion
	var err error
	if repoPath == "" {
		ret, err = chartVersionOCI(chart, version, callNum)
	} else {
		ret, err = chartVersionHTTP(chart, repoPath, callNum)
	}

	if err != nil {
		return nil, err
	}

	ret.Current = version
	return ret, nil
}

func chartVersionOCI(chartURI, version string, callNum int) (*ChartVersion, error) {
	regClient, err := registry.NewClient()
	if err != nil {
		return nil, err
	}
	tags, err := regClient.Tags(strings.TrimPrefix(chartURI, "oci://"))
	if err != nil {
		return nil, err
	}

	if len(tags) == 0 {
		return nil, fmt.Errorf("no tags found for chart %s", chartURI)
	}

	ret := &ChartVersion{
		Name:       chartURI,
		NewVersion: tags[0],
	}

	// Only check outdated dependencies if we're already on the latest version
	if version == tags[0] {
		chartYAML, err := downloadChartFile(chartURI, version, "", "Chart.yaml")
		if err != nil {
			return nil, err
		}

		var chartMeta chart.Metadata
		if err := yaml.Unmarshal(chartYAML, &chartMeta); err != nil {
			return nil, err
		}

		for _, dep := range chartMeta.Dependencies {
			latestDep, err := chartVersion(dep.Name, dep.Version, dep.Repository, 0)
			if err != nil {
				return nil, err
			}

			ret.Dependencies = append(ret.Dependencies, *latestDep)
		}
	}

	return ret, nil
}

func chartVersionHTTP(chart, repoPath string, callNum int) (*ChartVersion, error) {
	parsedURL, err := url.Parse(repoPath)
	if err != nil {
		return nil, err
	}
	parsedURL.RawPath = path.Join(parsedURL.RawPath, "index.yaml")
	parsedURL.Path = path.Join(parsedURL.Path, "index.yaml")

	resp, err := http.Get(parsedURL.String())
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	overview := struct {
		Entries map[string]ChartVersions `json:"entries"`
	}{}
	if err := yaml.NewDecoder(resp.Body).Decode(&overview); err != nil {
		return nil, err
	}

	versions, ok := overview.Entries[chart]
	if !ok {
		return nil, fmt.Errorf("chart %s not found in repository", chart)
	}

	ret := &ChartVersion{
		Name:       chart,
		NewVersion: versions[0].Version,
	}

	// Since we're not using a helm registry, we most likely can't update a chart manuall
	// so we ignore dependencies for now
	// for _, dep := range versions[0].Dependencies {
	// 	depChart, err := latestVersion(dep.Name, dep.Repository, 0)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	ret.Dependencies = append(ret.Dependencies, *depChart)
	// }

	return ret, nil
}

type ChartVersions []*chart.Metadata

// Len returns the length.
func (c ChartVersions) Len() int { return len(c) }

// Swap swaps the position of two items in the versions slice.
func (c ChartVersions) Swap(i, j int) { c[i], c[j] = c[j], c[i] }

// Less returns true if the version of entry a is less than the version of entry b.
func (c ChartVersions) Less(a, b int) bool {
	// Failed parse pushes to the back.
	i, err := semver.NewVersion(c[a].Version)
	if err != nil {
		return true
	}
	j, err := semver.NewVersion(c[b].Version)
	if err != nil {
		return false
	}
	return i.LessThan(j)
}
