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
	"k8s.io/client-go/kubernetes"
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
	log        *logrus.Entry
}

func NewConsoleManager(ConsoleSubscriber ConsoleReceiver, kubeClient kubernetes.Interface, log *logrus.Entry) *ConsoleManager {
	receiver := &ConsoleManager{
		Consoles:   ConsoleSubscriber,
		kubeClient: kubeClient,
		log:        log,
	}

	return receiver
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

	existing, err := c.kubeClient.CoreV1().Namespaces().Get(ctx, data.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err := c.kubeClient.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("creating namespace: %w", err)
			}
			log.Infof("Created namespace %s", data.Name)
			return nil
		}
		return fmt.Errorf("getting namespace: %w", err)
	}

	if metav1.HasAnnotation(existing.ObjectMeta, "cnrm.cloud.google.com/project-id") {
		return nil
	}

	if data.GCPProject == "" {
		return nil
	}

	metav1.SetMetaDataAnnotation(&existing.ObjectMeta, "cnrm.cloud.google.com/project-id", data.GCPProject)

	_, err = c.kubeClient.CoreV1().Namespaces().Update(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("updating namespace: %w", err)
	}
	log.Infof("Updated namespace %s with GCP project annotation", data.Name)
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
			log.Infof("Created service account %s in namespace %s", svcAccount.GetName(), svcAccount.GetNamespace())
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
				APIGroup:  "",
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

	existing, err := c.kubeClient.RbacV1().RoleBindings(roleBinding.GetNamespace()).Get(ctx, roleBinding.GetName(), metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err := c.kubeClient.RbacV1().RoleBindings(roleBinding.GetNamespace()).Create(ctx, &roleBinding, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("creating role binding: %w", err)
			}
			log.Infof("Created role binding %s in namespace %s", roleBinding.GetName(), roleBinding.GetNamespace())
		} else {
			return fmt.Errorf("getting role binding: %w", err)
		}
	} else {
		roleBinding.ObjectMeta = existing.ObjectMeta
		_, err := c.kubeClient.RbacV1().RoleBindings(roleBinding.GetNamespace()).Update(ctx, &roleBinding, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("update role binding: %w", err)
		}
		log.Infof("Updated role binding %s in namespace %s", roleBinding.GetName(), roleBinding.GetNamespace())
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
