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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
)

var now = time.Now

const (
	namespace    = "nais-system"
	defaultImage = "europe-north1-docker.pkg.dev/nais-io/nais/images/naisd"
	defaultTag   = "main"
)

func StartJob(ctx context.Context, client kubernetes.Interface, msg message.DeployInstruction, saName, naisProjectID, env, tenantName string) error {
	suffix := now().UTC().Format("20060102-150405")
	job := createJob(suffix, msg, saName, naisProjectID, env, tenantName)

	newJob, err := client.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("selfupgrade.Start: create job: %w", err)
	}

	secret, err := createSecretValues(suffix, msg, newJob.GetUID())
	if err != nil {
		return fmt.Errorf("selfupgrade.Start: generate secret: %w", err)
	}

	_, err = client.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("selfupgrade.Start: create secret: %w", err)
	}

	return nil
}

func createJob(suffix string, msg message.DeployInstruction, saName, naisProjectID, env, tenantName string) *batchv1.Job {
	lbls := map[string]string{
		"app": "naisd-self-upgrader",
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "naisd-self-upgrader-" + suffix,
			Labels: lbls,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            pointer.Int32(1),
			Completions:             pointer.Int32(1),
			TTLSecondsAfterFinished: pointer.Int32(int32((3 * time.Hour).Seconds())),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "naisd-self-upgrader",
							Image: image(msg.Values),
							Args: []string{
								"--production",
								"--nais-project-id",
								naisProjectID,
								"--env",
								env,
								"--tenant-name",
								tenantName,
								"upgrade",
							},
							SecurityContext: &corev1.SecurityContext{
								RunAsNonRoot:             pointer.Bool(true),
								AllowPrivilegeEscalation: pointer.Bool(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
								SeccompProfile: &corev1.SeccompProfile{
									Type: corev1.SeccompProfileTypeRuntimeDefault,
								},
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
					SecurityContext: &corev1.PodSecurityContext{
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
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

func createSecretValues(suffix string, values message.DeployInstruction, uid types.UID) (*corev1.Secret, error) {
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
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: batchv1.SchemeGroupVersion.String(),
					Kind:       "Job",
					Name:       "naisd-self-upgrader-" + suffix,
					UID:        uid,
				},
			},
		},
		StringData: map[string]string{
			"deploy_instruction.json": buf.String(),
		},
	}, nil
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
