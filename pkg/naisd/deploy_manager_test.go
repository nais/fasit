package naisd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/message"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestDeployReceiver(t *testing.T) {
	getEnvironment = func() []string {
		return []string{"FOO=bar"}
	}
	defer func() {
		getEnvironment = os.Environ
	}()

	diid := uuid.New()
	tests := map[string]struct {
		messages []message.DeployInstruction
		statuses []message.Status
		cmds     []cmd
	}{
		"no messages": {
			messages: []message.DeployInstruction{},
		},
		"helm_install": {
			messages: []message.DeployInstruction{
				{
					ID:         diid,
					Name:       "feature1",
					Version:    "1",
					Chart:      "chart1",
					ConfigHash: "hash1",
					Values:     map[string]any{"val1": "val1"},
				},
			},
			statuses: []message.Status{
				{
					Tenant:      "tenant1",
					Environment: "prod",
					Type:        2,
					Data:        []uint8(`{"DIID":"` + diid.String() + `","Name":"","Version":"","RolloutStatus":"pending","ConfigHash":"hash1","Log":""}`),
				},
				{
					Tenant:      "tenant1",
					Environment: "prod",
					Type:        2,
					Data:        []uint8(`{"DIID":"` + diid.String() + `","Name":"","Version":"","RolloutStatus":"deployed","ConfigHash":"hash1","Log":""}`),
				},
			},
			cmds: []cmd{
				{
					Args: []string{
						"helm", "upgrade",
						"--atomic",
						"--cleanup-on-fail",
						"--history-max", "10",
						"--install", "feature1", "chart1",
						"--namespace", "nais-system", "--create-namespace",
						"--version", "1", "-f", "/tmp/values.yaml",
						"--timeout", "5m0s",
						"--kube-apiserver", "somehost", "--kube-ca-file", "cafile",
						"--kube-token", "bearertoken",
					},
					Env: []string{"FOO=bar", "HELM_CACHE_HOME=/tmp/naisd-helm"},
				},
			},
		},
		"helm_uinstall": {
			messages: []message.DeployInstruction{
				{
					Name:      "feature13",
					Timeout:   10 * time.Minute,
					Uninstall: true,
				},
			},
			cmds: []cmd{
				{
					Args: []string{
						"helm", "uninstall",
						"--namespace", "nais-system",
						"--timeout", "10m0s",
						"feature13",
						"--kube-apiserver", "somehost", "--kube-ca-file", "cafile",
						"--kube-token", "bearertoken",
					},
					Env: []string{"FOO=bar", "HELM_CACHE_HOME=/tmp/naisd-helm"},
				},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			sub := &mockDeploymentReceiver{
				messages: tt.messages,
			}
			pub := &mockStatusPublisher{}
			executor := &mockExecutor{}
			dr, err := NewDeployManager(
				sub,
				pub,
				"tenant1",
				"prod",
				executor,
				nil, // k8s client only used with a deploy named `naisd`
				&rest.Config{
					Host:        "somehost",
					BearerToken: "bearertoken",
					TLSClientConfig: rest.TLSClientConfig{
						CAFile: "cafile",
					},
				},
				"naisd",
				"nais-project",
				logrus.NewEntry(logrus.StandardLogger()),
			)
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}

			ctx := context.Background()
			ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
			defer cancel()
			dr.runOnce()
			dr.helmCache = "/tmp/naisd-helm"
			dr.createTempFile = func(s1, s2 string) (file, error) {
				return &mockfile{}, nil
			}
			dr.Run(ctx)

			if !cmp.Equal(tt.statuses, pub.messages) {
				t.Error(cmp.Diff(tt.statuses, pub.messages))
			}
			if !cmp.Equal(tt.cmds, executor.cmds, cmpopts.IgnoreUnexported(exec.Cmd{})) {
				fmt.Printf("%#v\n", executor.cmds[0].Args)
				t.Error(cmp.Diff(tt.cmds, executor.cmds))
			}
		})
	}
}

func TestDeployReceiver_naisd_postpone_if_others_in_progress(t *testing.T) {
	sub := &mockDeploymentReceiver{}
	pub := &mockStatusPublisher{}
	dr, err := NewDeployManager(
		sub,
		pub,
		"tenant1",
		"prod",
		nil,
		nil, // k8s client only used with a deploy named `naisd`
		nil,
		"naisd",
		"nais-project",
		logrus.NewEntry(logrus.StandardLogger()),
	)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	// Simulate in progress deployments
	dr.inProgress.Inc()

	msg := message.DeployInstruction{
		Name:    "naisd",
		Version: "next",
	}
	err = dr.handler(context.Background(), msg)
	if err != errPostpone {
		t.Errorf("Expected errPostpone, got %v", err)
	}
}

func TestDeployReceiver_naisd_if_none_in_progress(t *testing.T) {
	sub := &mockDeploymentReceiver{}
	pub := &mockStatusPublisher{}
	executor := &mockExecutor{}
	dr, err := NewDeployManager(
		sub,
		pub,
		"tenant1",
		"prod",
		executor,
		mockNaisdK8s(),
		&rest.Config{
			Host:        "somehost",
			BearerToken: "bearertoken",
			TLSClientConfig: rest.TLSClientConfig{
				CAFile: "cafile",
			},
		},
		"naisd",
		"nais-project",
		logrus.NewEntry(logrus.StandardLogger()),
	)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	dr.runOnce()
	dr.helmCache = "/tmp/naisd-helm"
	dr.createTempFile = func(s1, s2 string) (file, error) {
		return &mockfile{}, nil
	}
	dr.stop = func() {}
	msg := message.DeployInstruction{
		Name:       "naisd",
		Version:    "next",
		Chart:      "chart1",
		ConfigHash: "hash1",
	}

	oldFunc := model.DownloadChartFunc
	defer func() { model.DownloadChartFunc = oldFunc }()
	model.DownloadChartFunc = func(chart, version, repo string) (*bytes.Buffer, error) {
		b, err := os.ReadFile("./selfupgrade/testdata/naisd.tgz")
		if err != nil {
			t.Fatal(err)
		}

		return bytes.NewBuffer(b), nil
	}

	err = dr.handler(ctx, msg)
	if err != errRestartRequired {
		t.Errorf("Expected errPostpone, got %v", err)
	}
}

type mockDeploymentReceiver struct {
	messages []message.DeployInstruction
}

func (m *mockDeploymentReceiver) Name() string {
	return "receiver"
}
func (m *mockDeploymentReceiver) Synchronous() {}
func (m *mockDeploymentReceiver) Receive(ctx context.Context, f func(ctx context.Context, msg message.DeployInstruction) error) error {
	for _, msg := range m.messages {
		if err := f(ctx, msg); err != nil {
			return err
		}
	}

	return nil
}

type mockStatusPublisher struct {
	messages []message.Status
}

func (m *mockStatusPublisher) Publish(ctx context.Context, msg message.Status) error {
	m.messages = append(m.messages, msg)
	return nil
}

type cmd struct {
	Args []string
	Env  []string
}

type mockExecutor struct {
	cmds []cmd
}

func (m *mockExecutor) Execute(c *exec.Cmd) error {
	m.cmds = append(m.cmds, cmd{
		Args: c.Args,
		Env:  c.Env,
	})
	return nil
}

type mockfile struct{}

func (m *mockfile) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func (m *mockfile) Close() error {
	return nil
}

func (m *mockfile) Name() string {
	return "/tmp/values.yaml"
}

func mockNaisdK8s() *fake.Clientset {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	return fake.NewSimpleClientset(&corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      hostname,
			Namespace: "nais-system",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "naisd",
				},
			},
		},
	})
}
