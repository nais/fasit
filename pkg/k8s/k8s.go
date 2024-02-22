package k8s

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/aquasecurity/trivy-operator/pkg/apis/aquasecurity/v1alpha1"
	"github.com/nais/fasit/pkg/database"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

type ClusterInformers map[string]*Informers

type Informers struct {
	ConfigAuditReportInformer           informers.GenericInformer
	ClusterComplianceReportInformer     informers.GenericInformer
	ClusterRbacAssessmentReportInformer informers.GenericInformer
}

func (c ClusterInformers) Start(ctx context.Context, log logrus.FieldLogger) error {
	for cluster, i := range c {
		log.WithField("cluster", cluster).Infof("starting informers")
		go i.ConfigAuditReportInformer.Informer().Run(ctx.Done())
		log.Info("started config audit informer")
		go i.ClusterComplianceReportInformer.Informer().Run(ctx.Done())
		log.Info("started cluster compliance informer")
		go i.ClusterRbacAssessmentReportInformer.Informer().Run(ctx.Done())
		log.Info("started cluster rbac informer")
	}

	for env, i := range c {
		if !i.ConfigAuditReportInformer.Informer().HasSynced() {
			log.Infof("waiting for config audit informer to sync in %q", env)
			select {
			case <-ctx.Done():
				return fmt.Errorf("informers not started: %w", ctx.Err())
			default:
				time.Sleep(2 * time.Second)
			}
		}
	}
	return nil
}

type Client struct {
	informers  ClusterInformers
	clientSets map[string]kubernetes.Interface
	log        logrus.FieldLogger
}

func (c *Client) Informers() ClusterInformers {
	return c.informers
}

func (c *Client) GetUnstructuredConfigAuditReports(env string) ([]runtime.Object, error) {
	return c.informers[env].ConfigAuditReportInformer.Lister().List(labels.Everything())
}

func (c *Client) GetUnstructuredClusterComplianceReports(env string) ([]runtime.Object, error) {
	return c.informers[env].ClusterComplianceReportInformer.Lister().List(labels.Everything())
}

func (c *Client) GetUnstructuredClusterRbacReports(env string) ([]runtime.Object, error) {
	return c.informers[env].ClusterRbacAssessmentReportInformer.Lister().List(labels.Everything())
}

type settings struct {
	clientsCreator func(cluster string) (kubernetes.Interface, dynamic.Interface, error)
}

type Opt func(*settings)

func WithClientsCreator(f func(cluster string) (kubernetes.Interface, dynamic.Interface, error)) Opt {
	return func(s *settings) {
		s.clientsCreator = f
	}
}

func New(ctx context.Context, repo database.Repo, log logrus.FieldLogger, opts ...Opt) (*Client, error) {
	s := &settings{}
	for _, opt := range opts {
		opt(s)
	}

	if s.clientsCreator == nil {
		restConfigs, err := CreateClusterConfigMap(ctx, repo)
		if err != nil {
			return nil, fmt.Errorf("create kubeconfig: %w", err)
		}

		s.clientsCreator = func(cluster string) (kubernetes.Interface, dynamic.Interface, error) {
			restConfig := restConfigs[cluster]
			clientSet, err := kubernetes.NewForConfig(&restConfig)
			if err != nil {
				return nil, nil, fmt.Errorf("create clientset: %w", err)
			}

			dynamicClient, err := dynamic.NewForConfig(&restConfig)
			if err != nil {
				return nil, nil, fmt.Errorf("create dynamic client: %w", err)
			}

			return clientSet, dynamicClient, nil
		}
	}

	infs := map[string]*Informers{}
	clientSets := map[string]kubernetes.Interface{}
	clusters, err := GetClusters(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("get clusters: %w", err)
	}
	for _, cluster := range clusters {
		infs[cluster] = &Informers{}

		clientSet, dynamicClient, err := s.clientsCreator(cluster)
		if err != nil {
			return nil, fmt.Errorf("create clientsets: %w", err)
		}

		log.WithField("cluster", cluster).Debug("creating informers")
		dinf := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 4*time.Hour)

		infs[cluster].ConfigAuditReportInformer = dinf.ForResource(v1alpha1.SchemeGroupVersion.WithResource("configauditreports"))
		infs[cluster].ClusterComplianceReportInformer = dinf.ForResource(v1alpha1.SchemeGroupVersion.WithResource("clustercompliancereports"))
		infs[cluster].ClusterRbacAssessmentReportInformer = dinf.ForResource(v1alpha1.SchemeGroupVersion.WithResource("rbacassessmentreports"))
		clientSets[cluster] = clientSet
	}

	return &Client{
		informers:  infs,
		log:        log,
		clientSets: clientSets,
	}, nil
}
