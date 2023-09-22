package selfupgrade

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/message"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
)

var now = time.Now

const (
	namespace = "nais-system"
	imageName = "europe-north1-docker.pkg.dev/nais-io/nais/images/naisd"
)

func StartJob(ctx context.Context, client kubernetes.Interface, msg message.DeployInstruction, naisProjectID, env, tenantName string) error {
	feature, err := model.FromChart(msg.Chart, msg.Version)
	if err != nil {
		return err
	}

	suffix := now().UTC().Format("20060102-150405")
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("unable to get hostname: %w", err)
	}
	podSpec, err := client.CoreV1().Pods(namespace).Get(ctx, hostname, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod spec: %w", err)
	}

	job := createJob(suffix, msg, naisProjectID, env, tenantName, podSpec.Spec, feature.ValuesYAML)

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

func createJob(suffix string, msg message.DeployInstruction, naisProjectID, env, tenantName string, spec corev1.PodSpec, vals map[string]json.RawMessage) *batchv1.Job {
	lbls := map[string]string{
		"app":                        "naisd-self-upgrader",
		"app.kubernetes.io/instance": "naisd",
	}
	container := corev1.Container{}
	containerIndex := 0
	for i, c := range spec.Containers {
		if c.Name == "naisd" {
			container = c
			containerIndex = i
		}
	}
	container.Args = []string{
		"--production",
		"--nais-project-id",
		naisProjectID,
		"--env",
		env,
		"--tenant-name",
		tenantName,
		"upgrade",
	}
	container.Image = image(vals)
	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      "instruction",
		ReadOnly:  true,
		MountPath: "/etc/naisd/self-upgrade",
	})
	spec.Containers[containerIndex] = container
	spec.RestartPolicy = corev1.RestartPolicyNever
	spec.Volumes = append(spec.Volumes, corev1.Volume{
		Name: "instruction",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: "naisd-self-upgrader-" + suffix,
			},
		},
	})

	spec.NodeName = ""
	spec.SchedulerName = ""

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "naisd-self-upgrader-" + suffix,
			Labels: lbls,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            ptr.To(int32(1)),
			Completions:             ptr.To(int32(1)),
			TTLSecondsAfterFinished: ptr.To(int32((3 * time.Hour).Seconds())),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: lbls,
				},
				Spec: spec,
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

func image(v map[string]json.RawMessage) string {
	tag := "latest"
	if err := json.Unmarshal(v["image.tag"], &tag); err != nil {
		log.Println("Error when getting image tag", err, "Using tag", tag)
	}

	return imageName + ":" + tag
}
