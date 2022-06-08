package naisd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nais/fasit/pkg/message"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type KubernetesReporter struct {
	client kubernetes.Interface
	pub    StatusPublisher
	tenant string
	env    string
}

func NewKubernetesReporter(tenant, env string, client kubernetes.Interface, pub StatusPublisher) *KubernetesReporter {
	return &KubernetesReporter{
		client: client,
		pub:    pub,
		tenant: tenant,
		env:    env,
	}
}

func (k *KubernetesReporter) Run(ctx context.Context) error {
	return k.reportNodes(ctx)
}

func (k *KubernetesReporter) reportNodes(ctx context.Context) error {
	nodes, err := k.nodeList(ctx)
	if err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	kn := message.KubernetesNodes{}
	for _, n := range nodes {
		kn.Nodes = append(kn.Nodes, k.createMessage(n))
	}

	hrb, err := json.Marshal(kn)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}

	return k.pub.Publish(ctx, message.Status{
		Tenant:      k.tenant,
		Environment: k.env,
		Type:        message.StatusKubernetesNodes,
		Data:        hrb,
	})
}

func (k *KubernetesReporter) nodeList(ctx context.Context) ([]corev1.Node, error) {
	var ret []corev1.Node
	next := ""
	for {
		nodes, err := k.client.CoreV1().Nodes().List(ctx, v1.ListOptions{
			Limit:    100,
			Continue: next,
		})
		if err != nil {
			return nil, err
		}

		ret = append(ret, nodes.Items...)
		next = nodes.GetListMeta().GetContinue()
		if next == "" {
			break
		}
	}

	return ret, nil
}

func (k *KubernetesReporter) createMessage(n corev1.Node) message.KubernetesNode {
	kn := message.KubernetesNode{
		Name:                    n.Name,
		KernelVersion:           n.Status.NodeInfo.KernelVersion,
		OSImage:                 n.Status.NodeInfo.OSImage,
		ContainerRuntimeVersion: n.Status.NodeInfo.ContainerRuntimeVersion,
		KubeletVersion:          n.Status.NodeInfo.KubeletVersion,
		KubeProxyVersion:        n.Status.NodeInfo.KubeProxyVersion,
		OperatingSystem:         n.Status.NodeInfo.OperatingSystem,
		Architecture:            n.Status.NodeInfo.Architecture,
		Allocatable:             k.nodeResource(n.Status.Allocatable),
		Capacity:                k.nodeResource(n.Status.Capacity),
	}

	for _, addr := range n.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			kn.InternalIP = addr.Address
			break
		}
	}

	for _, c := range n.Status.Conditions {
		kn.Conditions = append(kn.Conditions, message.KubernetesNodeCondition{
			Type:           message.KubernetesNodeConditionType(c.Type),
			Status:         message.ConditionStatus(c.Status),
			Reason:         c.Reason,
			Message:        c.Message,
			LastHeartbeat:  c.LastHeartbeatTime.Time,
			LastTransition: c.LastTransitionTime.Time,
		})
	}

	return kn
}

func (k *KubernetesReporter) nodeResource(rl corev1.ResourceList) message.KubernetesNodeResources {
	return message.KubernetesNodeResources{
		CPU:     rl.Cpu().Value(),
		Memory:  rl.Memory().Value(),
		Storage: rl.StorageEphemeral().Value(),
		Pods:    rl.Pods().Value(),
	}
}
