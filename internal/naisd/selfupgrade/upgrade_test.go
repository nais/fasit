package selfupgrade

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestFullRun(t *testing.T) {
	oldFunc := model.DownloadChartFunc
	defer func() { model.DownloadChartFunc = oldFunc }()
	model.DownloadChartFunc = func(chart, version, repo string) (*bytes.Buffer, error) {
		b, err := os.ReadFile("./testdata/naisd.tgz")
		if err != nil {
			t.Fatal(err)
		}

		return bytes.NewBuffer(b), nil
	}

	ctx := context.Background()
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hostname,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "naisd"}},
		},
	})

	deployInstruction := message.DeployInstruction{
		ID:         uuid.New(),
		Name:       "naisd",
		Version:    "1.2.3",
		Chart:      "oci://asdf",
		ConfigHash: "123",
		Timeout:    time.Minute,
	}

	now = func() time.Time {
		return time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	}

	err = StartJob(ctx, client, deployInstruction, "nais-project", "dev", "test-tenant")
	if err != nil {
		t.Fatal(err)
	}

	job, err := client.BatchV1().Jobs(namespace).Get(ctx, "naisd-self-upgrader-20200101-000000", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	secret, err := client.CoreV1().Secrets(namespace).Get(ctx, "naisd-self-upgrader-20200101-000000", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	jobMap := objToMap(t, job)
	secretMap := objToMap(t, secret)

	wantJob := map[string]any{
		"metadata": map[string]any{"creationTimestamp": nil, "labels": map[string]any{"app": string("naisd-self-upgrader"), "app.kubernetes.io/instance": string("naisd")}, "name": string("naisd-self-upgrader-20200101-000000"), "namespace": string("nais-system")},
		"spec": map[string]any{
			"backoffLimit": float64(1),
			"completions":  float64(1),
			"template": map[string]any{
				"metadata": map[string]any{
					"creationTimestamp": nil,
					"labels": map[string]any{
						"app":                        string("naisd-self-upgrader"),
						"app.kubernetes.io/instance": string("naisd"),
					},
				},
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"args": []any{
								string("--production"), string("--nais-project-id"), string("nais-project"), string("--env"), string("dev"),
								string("--tenant-name"),
								string("test-tenant"),
								string("upgrade"),
							},
							"image":        string("europe-north1-docker.pkg.dev/nais-io/nais/images/naisd:newtag"),
							"name":         string("naisd"),
							"resources":    map[string]any{},
							"volumeMounts": []any{map[string]any{"mountPath": string("/etc/naisd/self-upgrade"), "name": string("instruction"), "readOnly": true}},
						},
					},
					"restartPolicy": string("Never"),
					"volumes":       []any{map[string]any{"name": string("instruction"), "secret": map[string]any{"secretName": string("naisd-self-upgrader-20200101-000000")}}},
				},
			},
			"ttlSecondsAfterFinished": float64(10800),
		},
		"status": map[string]any{},
	}

	if !cmp.Equal(wantJob, jobMap) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(wantJob, jobMap))
	}

	wantSecret := map[string]any{
		"metadata": map[string]any{
			"creationTimestamp": nil,
			"name":              "naisd-self-upgrader-20200101-000000",
			"namespace":         "nais-system",
			"labels": map[string]any{
				"app": "naisd-self-upgrader",
			},
			"ownerReferences": []any{
				map[string]any{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"name":       "naisd-self-upgrader-20200101-000000",
					"uid":        "",
				},
			},
		},
		"stringData": map[string]any{
			"deploy_instruction.json": "{\"ID\":\"" + deployInstruction.ID.String() + "\",\"Name\":\"naisd\",\"Version\":\"1.2.3\",\"Chart\":\"oci://asdf\",\"ConfigHash\":\"123\",\"Timeout\":60000000000,\"Values\":null}\n",
		},
	}
	if !cmp.Equal(wantSecret, secretMap) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(wantSecret, secretMap))
	}
}

func Test_image(t *testing.T) {
	tests := map[string]struct {
		values map[string]json.RawMessage
		want   string
	}{
		"no values": {
			values: nil,
			want:   imageName + ":latest",
		},
		"with tag": {
			values: map[string]json.RawMessage{
				"image.tag": []byte(`"1.0.0"`),
			},
			want: imageName + ":1.0.0",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := image(tt.values); got != tt.want {
				t.Errorf("image() = %v, want %v", got, tt.want)
			}
		})
	}
}

func objToMap(tb testing.TB, obj interface{}) map[string]any {
	tb.Helper()
	b, err := json.Marshal(obj)
	if err != nil {
		tb.Fatal(err)
	}

	var m map[string]any
	err = json.Unmarshal(b, &m)
	if err != nil {
		tb.Fatal(err)
	}
	return m
}
