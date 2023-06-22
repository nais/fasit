package naisd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/nais/fasit/pkg/message"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

func TestDeployReceiver(t *testing.T) {
	getEnvironment = func() []string {
		return []string{"FOO=bar"}
	}
	defer func() {
		getEnvironment = os.Environ
	}()

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
					Data:        []uint8(`{"Name":"feature1","Version":"1","RolloutStatus":"pending","ConfigHash":"hash1","Log":""}`),
				},
				{
					Tenant:      "tenant1",
					Environment: "prod",
					Type:        2,
					Data:        []uint8(`{"Name":"feature1","Version":"1","RolloutStatus":"deployed","ConfigHash":"hash1","Log":""}`),
				},
			},
			cmds: []cmd{
				{
					Args: []string{
						"helm", "upgrade", "--atomic",
						"--install", "feature1", "chart1",
						"--namespace", "nais-system", "--create-namespace",
						"--version", "1", "-f", "/tmp/values.yaml",
						"--timeout", "5m0s",
						"--kube-apiserver", "somehost", "--kube-ca-file", "cafile",
						"--kube-token", "bearertoken", "--atomic", "--cleanup-on-fail",
					},
					Env: []string{"FOO=bar", "HELM_EXPERIMENTAL_OCI=1", "HELM_CACHE_HOME=/tmp/naisd-helm"},
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
			dr.helmCache = "/tmp/naisd-helm"
			dr.createTempFile = func(s1, s2 string) (file, error) {
				return &mockfile{}, nil
			}
			dr.Run(ctx)

			if !cmp.Equal(tt.statuses, pub.messages) {
				t.Errorf(cmp.Diff(tt.statuses, pub.messages))
			}
			if !cmp.Equal(tt.cmds, executor.cmds, cmpopts.IgnoreUnexported(exec.Cmd{})) {
				fmt.Printf("%#v\n", executor.cmds[0].Args)
				t.Errorf(cmp.Diff(tt.cmds, executor.cmds))
			}
		})
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
