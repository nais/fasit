package selfupgrade

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nais/fasit/pkg/message"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var now = time.Now

const (
	namespace    = "nais-system"
	defaultImage = "europe-north1-docker.pkg.dev/nais-io/nais/images/naisd"
	defaultTag   = "main"
)

func StartJob(ctx context.Context, client kubernetes.Interface, msg message.DeployInstruction, saName string) error {
	suffix := now().UTC().Format("20060102-150405")
	job := createJob(suffix, msg, saName)
	secret, err := createSecretValues(suffix, msg)
	if err != nil {
		return fmt.Errorf("selfupgrade.Start: generate secret: %w", err)
	}

	_, err = client.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("selfupgrade.Start: create secret: %w", err)
	}

	_, err = client.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("selfupgrade.Start: create job: %w", err)
	}
	return nil
}

func createJob(suffix string, msg message.DeployInstruction, saName string) *batchv1.Job {
	lbls := map[string]string{
		"app": "naisd-self-upgrader",
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "naisd-self-upgrader-" + suffix,
			Labels: lbls,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "naisd-self-upgrader",
							Image: image(msg.Values),
							Args: []string{
								"upgrade",
								"--production",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "instruction",
									ReadOnly:  true,
									MountPath: "/etc/naisd/self-upgrade",
								},
							},
						},
					},
					ServiceAccountName: saName,
					RestartPolicy:      corev1.RestartPolicyNever,
					Volumes: []corev1.Volume{
						{
							Name: "instruction",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "naisd-self-upgrader-" + suffix,
								},
							},
						},
					},
				},
			},
		},
	}
}

func createSecretValues(suffix string, values message.DeployInstruction) (*corev1.Secret, error) {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	if err := enc.Encode(values); err != nil {
		return nil, err
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "naisd-self-upgrader-" + suffix,
			Labels: map[string]string{
				"app": "naisd-self-upgrader",
			},
		},
		StringData: map[string]string{
			"deploy_instruction.json": buf.String(),
		},
	}, nil
}

func Cleanup(ctx context.Context, client kubernetes.Interface) error {
	jobs, err := client.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=naisd-self-upgrader",
	})
	if err != nil {
		return err
	}
	for _, job := range jobs.Items {
		err := client.BatchV1().Jobs(namespace).Delete(ctx, job.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	secrets, err := client.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=naisd-self-upgrader",
	})
	if err != nil {
		return err
	}

	for _, secret := range secrets.Items {
		err := client.CoreV1().Secrets(namespace).Delete(ctx, secret.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func imageTag(v map[string]any) string {
	image, ok := v["image"].(map[string]any)
	if !ok {
		return defaultTag
	}
	tag, ok := image["tag"].(string)
	if !ok {
		return defaultTag
	}
	return tag
}

func imageRepo(v map[string]any) string {
	image, ok := v["image"].(map[string]any)
	if !ok {
		return defaultImage
	}
	tag, ok := image["repository"].(string)
	if !ok {
		return defaultImage
	}
	return tag
}

func image(v map[string]any) string {
	return imageRepo(v) + ":" + imageTag(v)
}
