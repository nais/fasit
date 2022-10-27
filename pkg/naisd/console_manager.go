package naisd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nais/fasit/pkg/message"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var ErrDeleteRequiredNamespace = fmt.Errorf("namespace is required, cannot be deleted")

var cnrmConfigGroupVersionResource = schema.GroupVersionResource{
	Group:    "core.cnrm.cloud.google.com",
	Version:  "v1beta1",
	Resource: "configconnectorcontexts",
}

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
}

func NewConsoleManager(ConsoleSubscriber ConsoleReceiver, config *rest.Config, projectID string, log *logrus.Entry) (*ConsoleManager, error) {
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes client: %w", err)
	}
	dyncClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}
	receiver := &ConsoleManager{
		Consoles:   ConsoleSubscriber,
		kubeClient: kubeClient,
		dynClient:  dyncClient,
		log:        log,
		projectID:  projectID,
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

	if data.GCPProject != "" {
		metav1.SetMetaDataAnnotation(&ns.ObjectMeta, "cnrm.cloud.google.com/project-id", data.GCPProject)
	}

	metav1.SetMetaDataLabel(&ns.ObjectMeta, "team", data.Name)

	existing, err := c.kubeClient.CoreV1().Namespaces().Get(ctx, data.Name, metav1.GetOptions{})
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

	switch {
	case !metav1.HasAnnotation(existing.ObjectMeta, "cnrm.cloud.google.com/project-id") && data.GCPProject != "":
	case !metav1.HasLabel(existing.ObjectMeta, "team"):
	default:
		return nil
	}

	metav1.SetMetaDataAnnotation(&existing.ObjectMeta, "cnrm.cloud.google.com/project-id", data.GCPProject)
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

	_, err := c.kubeClient.CoreV1().ServiceAccounts(svcAccount.GetNamespace()).Get(ctx, svcAccount.GetName(), metav1.GetOptions{})
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

	roleBinding := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("team-%s-naisdeveloper", data.Name),
			Namespace: data.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "Group",
				Name: data.GroupEmail,
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

func (c *ConsoleManager) createOrUpdateRoleBinding(ctx context.Context, roleBinding rbacv1.RoleBinding, log logrus.FieldLogger) error {
	existing, err := c.kubeClient.RbacV1().RoleBindings(roleBinding.GetNamespace()).Get(ctx, roleBinding.GetName(), metav1.GetOptions{})
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
		return fmt.Errorf("unmarshal create namespace: %w", err)
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
	cnrmClient := c.dynClient.Resource(cnrmConfigGroupVersionResource).Namespace(data.Name)

	const contextName = "configconnectorcontext.core.cnrm.cloud.google.com"
	saEmail := "cnrm-" + data.Name + "@" + c.projectID + ".iam.gserviceaccount.com"

	res, err := cnrmClient.Get(ctx, contextName, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("getting config connector context: %w", err)
		}
		_, err := cnrmClient.Create(ctx, &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "core.cnrm.cloud.google.com/v1beta1",
				"kind":       "ConfigConnectorContext",
				"metadata": map[string]interface{}{
					"name": contextName,
				},
				"spec": map[string]interface{}{
					"googleServiceAccount": saEmail,
				},
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("creating CNRM config: %w", err)
		}
		return nil
	}

	res.Object["spec"] = map[string]interface{}{
		"googleServiceAccount": saEmail,
	}

	_, err = cnrmClient.Update(ctx, res, metav1.UpdateOptions{})
	return err
}
