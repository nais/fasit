package helm

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"

	memory "k8s.io/client-go/discovery/cached"
)

type Client struct {
	cfg *action.Configuration
}

func New(restConfig *rest.Config, namespace string, log *logrus.Entry) *Client {
	cfg := &action.Configuration{}

	// Init cannot error, it will panic if something goes wrong
	_ = cfg.Init(&K8sClient{cfg: restConfig}, namespace, "", log.Debugf)
	return &Client{
		cfg: cfg,
	}
}

type ChartVersion struct {
	Name         string
	NewVersion   string `json:"version"`
	Current      string
	Dependencies []ChartVersion `json:"dependencies"`
}

func (c *Client) Pull(version, chartRef string) error {
	p := action.NewPull()
	p.Settings = &cli.EnvSettings{}
	p.Version = version
	p.DestDir = "/tmp"
	r, err := p.Run(chartRef)
	fmt.Println("Chart pulled: ", r)
	if err != nil {
		fmt.Println("Error pulling chart: ", err)
		return err
	}

	return nil
}

func (c *Client) List() ([]*release.Release, error) {
	return action.NewList(c.cfg).Run()
}

type K8sClient struct {
	cfg *rest.Config
}

// ToRESTConfig returns restconfig
func (k *K8sClient) ToRESTConfig() (*rest.Config, error) {
	return k.cfg, nil
}

// ToDiscoveryClient returns discovery client
func (k *K8sClient) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	dcfg, err := discovery.NewDiscoveryClientForConfig(k.cfg)
	if err != nil {
		return nil, err
	}
	return memory.NewMemCacheClient(dcfg), nil
}

// ToRESTMapper returns a restmapper
func (k *K8sClient) ToRESTMapper() (meta.RESTMapper, error) {
	discoveryClient, err := k.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	expander := restmapper.NewShortcutExpander(mapper, discoveryClient)
	return expander, nil
}

// ToRawKubeConfigLoader return kubeconfig loader as-is
func (k *K8sClient) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	panic("not implemented")
}
