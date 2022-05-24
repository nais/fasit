package naisd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nais/fasit/pkg/message"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

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

func (d *ConsoleManager) Run(ctx context.Context) {
	d.log.WithField("subscription", d.Consoles.Name()).Info("Starting Console receiver")
	d.Consoles.Synchronous()
	err := d.Consoles.Receive(ctx, d.handler)
	if err != nil {
		d.log.WithError(err).Error("receive console messages")
		// retry logic, kanskje. Denne skal aldri trigge
	}
}

func (d *ConsoleManager) handler(ctx context.Context, msg message.Console) error {
	log := d.log.WithFields(logrus.Fields{
		"type": msg.Type,
	})

	log.Debug("Received console instruction")

	switch msg.Type {
	case message.ConsoleTypeCreateNamespace:
		return d.createNamespace(ctx, msg)
	default:
		log.Warn("Unknown console instruction")
	}

	return nil
}

func (c *ConsoleManager) createNamespace(ctx context.Context, msg message.Console) error {
	data := message.CreateNamespace{}
	err := json.Unmarshal(msg.Data, &data)
	if err != nil {
		return fmt.Errorf("unmarshal create namespace: %w", err)
	}

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
	return nil
}
