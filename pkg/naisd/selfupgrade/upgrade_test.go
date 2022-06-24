package selfupgrade

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/fasit/pkg/message"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestFullRun(t *testing.T) {
	client := fake.NewSimpleClientset()
	testCreate(t, client)
	testCleanup(t, client)
}

func testCreate(t *testing.T, client *fake.Clientset) {
	ctx := context.Background()

	deployInstruction := message.DeployInstruction{
		Name:       "naisd",
		Version:    "1.2.3",
		Chart:      "oci://asdf",
		ConfigHash: "123",
		Timeout:    time.Minute,
		Values: map[string]any{
			"image": map[string]any{
				"tag": "newtag",
			},
		},
	}

	serviceAccount := "naisd-service-account"

	now = func() time.Time {
		return time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	}

	err := StartJob(ctx, client, deployInstruction, serviceAccount)
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
		"metadata": map[string]any{
			"creationTimestamp": nil,
			"name":              "naisd-self-upgrader-20200101-000000",
			"namespace":         "nais-system",
			"labels": map[string]any{
				"app": "naisd-self-upgrader",
			},
		},
		"spec": map[string]any{
			"template": map[string]any{
				"metadata": map[string]any{"creationTimestamp": nil},
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":  "naisd-self-upgrader",
							"image": "europe-north1-docker.pkg.dev/nais-io/nais/images/naisd:newtag",
							"args": []any{
								"upgrade",
							},
							"resources": map[string]any{},
							"volumeMounts": []any{
								map[string]any{
									"mountPath": "/etc/naisd/self-upgrade",
									"name":      "instruction",
									"readOnly":  true,
								},
							},
						},
					},
					"volumes": []any{
						map[string]any{
							"name":   "instruction",
							"secret": map[string]any{"secretName": "naisd-self-upgrader-20200101-000000"},
						},
					},
					"restartPolicy":      "Never",
					"serviceAccountName": serviceAccount,
				},
			},
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
		},
		"stringData": map[string]any{
			"deploy_instruction.json": "{\"Name\":\"naisd\",\"Version\":\"1.2.3\",\"Chart\":\"oci://asdf\",\"Repo\":\"\",\"ConfigHash\":\"123\",\"Timeout\":60000000000,\"Values\":{\"image\":{\"tag\":\"newtag\"}}}\n",
		},
	}
	if !cmp.Equal(wantSecret, secretMap) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(wantSecret, secretMap))
	}
}

func testCleanup(t *testing.T, client *fake.Clientset) {
	ctx := context.Background()

	err := Cleanup(ctx, client)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.BatchV1().Jobs(namespace).Get(ctx, "naisd-self-upgrader-20200101-000000", metav1.GetOptions{})
	if !errors.IsNotFound(err) {
		t.Errorf("Job should have been deleted, got err: %v", err)
	}

	_, err = client.CoreV1().Secrets(namespace).Get(ctx, "naisd-self-upgrader-20200101-000000", metav1.GetOptions{})
	if !errors.IsNotFound(err) {
		t.Error("Secret should have been deleted, got err:", err)
	}
}

func Test_image(t *testing.T) {
	tests := map[string]struct {
		values map[string]any
		want   string
	}{
		"no values": {
			values: nil,
			want:   defaultImage + ":" + defaultTag,
		},
		"with tag": {
			values: map[string]any{
				"image": map[string]any{
					"tag": "1.0.0",
				},
			},
			want: defaultImage + ":1.0.0",
		},
		"with repository": {
			values: map[string]any{
				"image": map[string]any{
					"repository": "naisd/self-upgrader",
				},
			},
			want: "naisd/self-upgrader:" + defaultTag,
		},
		"with tag and repository": {
			values: map[string]any{
				"image": map[string]any{
					"tag":        "1.0.0",
					"repository": "naisd/self-upgrader",
				},
			},
			want: "naisd/self-upgrader:1.0.0",
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
