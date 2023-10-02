package naisd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	cnrmbeta1 "github.com/GoogleCloudPlatform/k8s-config-connector/operator/pkg/apis/core/v1beta1"
	"github.com/google/go-cmp/cmp"
	"github.com/nais/fasit/pkg/message"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1lister "k8s.io/client-go/listers/core/v1"
	rbacv1lister "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

var ErrDeleteRequiredNamespace = fmt.Errorf("namespace is required, cannot be deleted")

type ConsoleReceiver interface {
	Name() string
	Synchronous()
	Receive(ctx context.Context, f func(ctx context.Context, msg message.Console) error) error
}

type ConsoleManager struct {
	Consoles   ConsoleReceiver
	kubeClient kubernetes.Interface
	dynClient  dynamic.Interface
	projectID  string
	log        *logrus.Entry
	env        string
	nsList     v1lister.NamespaceLister
	saList     v1lister.ServiceAccountLister
	rbList     rbacv1lister.RoleBindingLister
	cnrmConfig cache.GenericLister
}

func NewConsoleManager(ctx context.Context, ConsoleSubscriber ConsoleReceiver, config *rest.Config, projectID string, envName string, log *logrus.Entry) (*ConsoleManager, error) {
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes client: %w", err)
	}
	dyncClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	return newConsoleManager(ctx, kubeClient, dyncClient, ConsoleSubscriber, config, projectID, envName, log)
}

func newConsoleManager(ctx context.Context,
	kubeClient kubernetes.Interface,
	dyncClient dynamic.Interface,
	ConsoleSubscriber ConsoleReceiver,
	config *rest.Config,
	projectID string,
	envName string,
	log *logrus.Entry,
) (*ConsoleManager, error) {
	inf := informers.NewSharedInformerFactory(kubeClient, 4*time.Hour)
	nsInf := inf.Core().V1().Namespaces()
	saInf := inf.Core().V1().ServiceAccounts()
	rbInf := inf.Rbac().V1().RoleBindings()
	go nsInf.Informer().Run(ctx.Done())
	go saInf.Informer().Run(ctx.Done())
	go rbInf.Informer().Run(ctx.Done())

	var cnrmLister cache.GenericLister
	if !strings.HasSuffix(envName, "-fss") {
		log.Info("Skipping setup of CNRM informer")
		dynInf := dynamicinformer.NewDynamicSharedInformerFactory(dyncClient, 4*time.Hour)
		cnrmInf := dynInf.ForResource(cnrmbeta1.GroupVersion.WithResource("configconnectorcontexts"))
		go cnrmInf.Informer().Run(ctx.Done())
		cnrmLister = cnrmInf.Lister()
	}

	// wait for caches to sync
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if !waitForCacheSync(ctx.Done(), nsInf.Informer().HasSynced, saInf.Informer().HasSynced, rbInf.Informer().HasSynced) {
		log.Warn("failed to sync caches")
	}

	receiver := &ConsoleManager{
		Consoles:   ConsoleSubscriber,
		kubeClient: kubeClient,
		dynClient:  dyncClient,
		log:        log,
		projectID:  projectID,
		env:        envName,
		nsList:     nsInf.Lister(),
		saList:     saInf.Lister(),
		rbList:     rbInf.Lister(),
		cnrmConfig: cnrmLister,
	}

	return receiver, nil
}

func (c *ConsoleManager) Run(ctx context.Context) {
	c.log.WithField("subscription", c.Consoles.Name()).Info("Starting Console receiver")
	c.Consoles.Synchronous()
	err := c.Consoles.Receive(ctx, c.handler)
	if err != nil {
		c.log.WithError(err).Error("receive console messages")
		// retry logic, kanskje. Denne skal aldri trigge
	}
}

func (c *ConsoleManager) handler(ctx context.Context, msg message.Console) error {
	log := c.log.WithFields(logrus.Fields{
		"type": msg.Type,
	})

	log.Debug("Received console instruction")

	switch msg.Type {
	case message.ConsoleTypeCreateNamespace:
		return c.create(ctx, msg, log)
	case message.ConsoleTypeDeleteNamespace:
		return c.deleteNamespace(ctx, msg)
	default:
		log.Warn("Unknown console instruction")
	}

	return nil
}

func (c *ConsoleManager) create(ctx context.Context, msg message.Console, log logrus.FieldLogger) error {
	data := message.CreateNamespace{}
	err := json.Unmarshal(msg.Data, &data)
	if err != nil {
		return fmt.Errorf("unmarshal create namespace: %w", err)
	}

	err = c.createNamespace(ctx, data, log)
	if err != nil {
		return err
	}

	err = c.createServiceAccounts(ctx, data, log)
	if err != nil {
		return err
	}

	err = c.createCNRMConfig(ctx, data, log)
	if err != nil {
		return err
	}

	err = c.createTeamRolebindings(ctx, data, log)
	if err != nil {
		return err
	}

	return nil
}

func (c *ConsoleManager) createNamespace(ctx context.Context, data message.CreateNamespace, log logrus.FieldLogger) error {
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: data.Name,
		},
	}

	metav1.SetMetaDataAnnotation(&ns.ObjectMeta, "linkerd.io/inject", "true")

	if data.GCPProject != "" {
		metav1.SetMetaDataAnnotation(&ns.ObjectMeta, "cnrm.cloud.google.com/project-id", data.GCPProject)
	}

	if data.SlackAlertsChannel != "" {
		metav1.SetMetaDataAnnotation(&ns.ObjectMeta, "replicator.nais.io/slackAlertsChannel", data.SlackAlertsChannel)
	}

	metav1.SetMetaDataLabel(&ns.ObjectMeta, "team", data.Name)

	existing, err := c.nsList.Get(data.Name)
	if err != nil {
		if errors.IsNotFound(err) {
			_, err := c.kubeClient.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("creating namespace: %w", err)
			}
			log.WithField("name", data.Name).Debug("Created namespace")
			return nil
		}
		return fmt.Errorf("getting namespace: %w", err)
	}

	existing = existing.DeepCopy()

	// cases where we should update the namespace.
	switch {
	case existing.Annotations == nil:
	case existing.Annotations["cnrm.cloud.google.com/project-id"] != data.GCPProject:
	case existing.Annotations["replicator.nais.io/slackAlertsChannel"] != data.SlackAlertsChannel:
	case existing.Annotations["linkerd.io/inject"] != "true":
	case existing.Labels == nil:
	case existing.Labels["team"] != data.Name:
	default:
		// no changes we care about, return
		return nil
	}

	metav1.SetMetaDataAnnotation(&existing.ObjectMeta, "cnrm.cloud.google.com/project-id", data.GCPProject)
	metav1.SetMetaDataAnnotation(&existing.ObjectMeta, "replicator.nais.io/slackAlertsChannel", data.SlackAlertsChannel)
	metav1.SetMetaDataAnnotation(&existing.ObjectMeta, "linkerd.io/inject", "true")
	metav1.SetMetaDataLabel(&existing.ObjectMeta, "team", data.Name)

	_, err = c.kubeClient.CoreV1().Namespaces().Update(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("updating namespace: %w", err)
	}
	log.WithField("ns", data.Name).Debug("Updated namespace")
	return nil
}

func (c *ConsoleManager) createServiceAccounts(ctx context.Context, data message.CreateNamespace, log logrus.FieldLogger) error {
	svcAccount := v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("serviceuser-%s", data.Name),
			Namespace: data.Name,
		},
	}

	_, err := c.saList.ServiceAccounts(svcAccount.GetNamespace()).Get(svcAccount.GetName())
	if err != nil {
		if errors.IsNotFound(err) {
			_, err := c.kubeClient.CoreV1().ServiceAccounts(svcAccount.GetNamespace()).Create(ctx, &svcAccount, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("creating service account: %w", err)
			}
			log.WithFields(logrus.Fields{
				"name": svcAccount.GetName(),
				"ns":   svcAccount.GetNamespace(),
			}).Debug("Created service account")
		} else {
			return fmt.Errorf("getting service account: %w", err)
		}
	}

	roleBinding := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("serviceuser-%s-naisdeveloper", data.Name),
			Namespace: data.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      svcAccount.GetName(),
				Namespace: svcAccount.GetNamespace(),
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "nais:developer",
		},
	}

	return c.createOrUpdateRoleBinding(ctx, roleBinding, log)
}

func (c *ConsoleManager) createTeamRolebindings(ctx context.Context, data message.CreateNamespace, log logrus.FieldLogger) error {
	if data.GroupEmail == "" {
		log.WithFields(logrus.Fields{
			"ns": data.Name,
		}).Warn("Unable to create team rolebinding, missing group email")
		return nil
	}

	subjects := []rbacv1.Subject{
		{
			Kind: "Group",
			Name: data.GroupEmail,
		},
	}

	if data.AzureGroupID != "" {
		subjects = append(subjects, rbacv1.Subject{
			Kind: "Group",
			Name: data.AzureGroupID,
		})
	}

	roleBinding := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("team-%s-naisdeveloper", data.Name),
			Namespace: data.Name,
		},
		Subjects: subjects,
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "nais:developer",
		},
	}

	return c.createOrUpdateRoleBinding(ctx, roleBinding, log)
}

func (c *ConsoleManager) createOrUpdateRoleBinding(ctx context.Context, roleBinding rbacv1.RoleBinding, log logrus.FieldLogger) error {
	existing, err := c.rbList.RoleBindings(roleBinding.GetNamespace()).Get(roleBinding.GetName())
	if err != nil {
		if errors.IsNotFound(err) {
			_, err := c.kubeClient.RbacV1().RoleBindings(roleBinding.GetNamespace()).Create(ctx, &roleBinding, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("creating role binding: %w", err)
			}
			log.WithFields(logrus.Fields{
				"name": roleBinding.GetName(),
				"ns":   roleBinding.GetNamespace(),
			}).Debug("Created role binding")
		} else {
			return fmt.Errorf("getting role binding: %w", err)
		}
	} else {
		roleBinding.ObjectMeta = existing.ObjectMeta

		if cmp.Equal(roleBinding.RoleRef, existing.RoleRef) && cmp.Equal(roleBinding.Subjects, existing.Subjects) {
			log.WithFields(logrus.Fields{
				"name": roleBinding.GetName(),
				"ns":   roleBinding.GetNamespace(),
			}).Debug("no changes to role binding")
			return nil
		}
		_, err := c.kubeClient.RbacV1().RoleBindings(roleBinding.GetNamespace()).Update(ctx, &roleBinding, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("update role binding: %w", err)
		}
		log.WithFields(logrus.Fields{
			"name": roleBinding.GetName(),
			"ns":   roleBinding.GetNamespace(),
		}).Debug("Updated role binding")
	}
	return nil
}

func (c *ConsoleManager) deleteNamespace(ctx context.Context, msg message.Console) error {
	data := message.DeleteNamespace{}
	err := json.Unmarshal(msg.Data, &data)
	if err != nil {
		return fmt.Errorf("unmarshal delete namespace: %w", err)
	}

	switch data.Name {
	case "nais-system",
		"kube-system",
		"default",
		"kube-public":
		c.log.WithField("namespace", data.Name).Warn("Namespace is not allowed to be deleted")
		return ErrDeleteRequiredNamespace
	}

	err = c.kubeClient.CoreV1().Namespaces().Delete(ctx, data.Name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return nil
}

func (c *ConsoleManager) createCNRMConfig(ctx context.Context, data message.CreateNamespace, log logrus.FieldLogger) error {
	if strings.HasSuffix(c.env, "-fss") {
		c.log.Info("Skipping CNRM config for FSS")
		return nil
	}
	cnrmClient := c.dynClient.Resource(cnrmbeta1.GroupVersion.WithResource("configconnectorcontexts")).Namespace(data.Name)

	const contextName = "configconnectorcontext.core.cnrm.cloud.google.com"

	// res, err := cnrmClient.Get(ctx, contextName, metav1.GetOptions{})
	res, err := c.cnrmConfig.ByNamespace(data.Name).Get(contextName)
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("getting config connector context: %w", err)
		}
		_, err := cnrmClient.Create(ctx, &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "core.cnrm.cloud.google.com/v1beta1",
				"kind":       "ConfigConnectorContext",
				"metadata": map[string]any{
					"name": contextName,
				},
				"spec": map[string]any{
					"googleServiceAccount": data.CNRMEmail,
				},
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("creating CNRM config: %w", err)
		}
		return nil
	}

	obj := res.DeepCopyObject().(*unstructured.Unstructured)

	// Check if we need to update the CNRM email
	if spec, ok := obj.Object["spec"]; ok {
		if specMap, ok := spec.(map[string]any); ok {
			if specMap["googleServiceAccount"] == data.CNRMEmail {
				return nil
			}
		}
	}
	obj.Object["spec"] = map[string]any{
		"googleServiceAccount": data.CNRMEmail,
	}

	_, err = cnrmClient.Update(ctx, obj, metav1.UpdateOptions{})
	return err
}

func waitForCacheSync(stop <-chan struct{}, cacheSyncs ...cache.InformerSynced) bool {
	max := time.Millisecond * 100
	delay := time.Millisecond
	f := func() bool {
		for _, syncFunc := range cacheSyncs {
			if !syncFunc() {
				return false
			}
		}
		return true
	}
	for {
		select {
		case <-stop:
			return false
		default:
		}
		res := f()
		if res {
			return true
		}
		delay *= 2
		if delay > max {
			delay = max
		}

		select {
		case <-stop:
			return false
		case <-time.After(delay):
		}
	}
}
