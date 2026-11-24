package daemon

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/nais/fasit/internal/fasitd/protogen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type fakeStream struct {
	grpc.ClientStream
	ctx    context.Context
	recvCh chan *protogen.ServerMessage

	mu   sync.Mutex
	sent []*protogen.AgentMessage
}

func (f *fakeStream) Send(msg *protogen.AgentMessage) error {
	if err := f.ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, msg)
	return nil
}

func (f *fakeStream) Recv() (*protogen.ServerMessage, error) {
	select {
	case <-f.ctx.Done():
		return nil, f.ctx.Err()
	case msg, ok := <-f.recvCh:
		if !ok {
			return nil, io.EOF
		}
		return msg, nil
	}
}

func (f *fakeStream) Context() context.Context { return f.ctx }

func (f *fakeStream) sentStatuses() []*protogen.CommandStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*protogen.CommandStatus
	for _, m := range f.sent {
		if s := m.GetStatus(); s != nil {
			out = append(out, s)
		}
	}
	return out
}

func (f *fakeStream) sentDrainingCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, m := range f.sent {
		if m.GetDraining() != nil {
			n++
		}
	}
	return n
}

type fakeClient struct {
	stream *fakeStream
}

func (c *fakeClient) Connect(ctx context.Context, _ ...grpc.CallOption) (grpc.BidiStreamingClient[protogen.AgentMessage, protogen.ServerMessage], error) {
	if _, ok := metadata.FromOutgoingContext(ctx); !ok {
		return nil, errors.New("missing outgoing metadata")
	}
	c.stream.ctx = ctx
	return c.stream, nil
}

type fakeExecutor struct {
	mu           sync.Mutex
	deployReqs   []DeployRequest
	uninstallReq []string
	deployFn     func(ctx context.Context, req DeployRequest, logFn func(string)) error
	uninstallFn  func(ctx context.Context, name string, timeout time.Duration, logFn func(string)) error
}

func (e *fakeExecutor) InstallOrUpgrade(ctx context.Context, req DeployRequest, logFn func(string)) error {
	e.mu.Lock()
	e.deployReqs = append(e.deployReqs, req)
	fn := e.deployFn
	e.mu.Unlock()
	if fn != nil {
		return fn(ctx, req, logFn)
	}
	return nil
}

func (e *fakeExecutor) Uninstall(ctx context.Context, name string, timeout time.Duration, logFn func(string)) error {
	e.mu.Lock()
	e.uninstallReq = append(e.uninstallReq, name)
	fn := e.uninstallFn
	e.mu.Unlock()
	if fn != nil {
		return fn(ctx, name, timeout, logFn)
	}
	return nil
}

func TestAgentSelfUpgradeDrainsOnSIGTERMAndRunsAtomic(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})

	exec := &fakeExecutor{
		deployFn: func(_ context.Context, req DeployRequest, logFn func(string)) error {
			logFn("upgrading fasitd")
			close(started)
			<-release
			return nil
		},
	}

	stream := &fakeStream{recvCh: make(chan *protogen.ServerMessage, 1)}
	client := &fakeClient{stream: stream}
	agent := NewAgent(client, AgentOptions{
		Tenant:      "nav",
		Environment: "dev",
		Version:     "1.0.0",
		Executor:    exec,
		DryRun:      false,
	}, slog.New(slog.DiscardHandler))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- agent.Run(ctx) }()

	// Wait for registration & readiness.
	deadline := time.Now().Add(2 * time.Second)
	for !agent.Ready() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !agent.Ready() {
		t.Fatalf("agent should be ready after registering")
	}

	// Send self-upgrade command.
	stream.recvCh <- &protogen.ServerMessage{
		Message: &protogen.ServerMessage_Command{
			Command: &protogen.Command{
				Diid:       "diid-1",
				Name:       "fasitd",
				Version:    "1.1.0",
				Chart:      "oci://example/fasitd",
				ConfigHash: "abc",
				Values:     []byte(`{"image":{"tag":"1.1.0"}}`),
			},
		},
	}

	<-started

	// Simulate Kubernetes SIGTERM once the new pod passes /readyz.
	cancel()
	time.Sleep(20 * time.Millisecond)
	if agent.Ready() {
		t.Errorf("agent should report not ready after SIGTERM")
	}

	// Stream must still be open so the in-flight Helm upgrade can finish and
	// report terminal status.
	close(release)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("agent.Run did not exit after in-flight command completed")
	}

	if len(exec.deployReqs) != 1 {
		t.Fatalf("deployReqs = %d, want 1", len(exec.deployReqs))
	}
	if !exec.deployReqs[0].Atomic {
		t.Errorf("self-upgrade must set Atomic=true")
	}

	statuses := stream.sentStatuses()
	if len(statuses) != 1 || statuses[0].GetStatus() != "deployed" {
		t.Errorf("sent statuses = %v, want single 'deployed'", statuses)
	}
	if got := stream.sentDrainingCount(); got != 1 {
		t.Errorf("sent draining count = %d, want 1", got)
	}
}

func TestAgentNonSelfUpgradeIsNotAtomic(t *testing.T) {
	exec := &fakeExecutor{}
	stream := &fakeStream{recvCh: make(chan *protogen.ServerMessage, 1)}
	agent := NewAgent(&fakeClient{stream: stream}, AgentOptions{
		Tenant:      "nav",
		Environment: "dev",
		Executor:    exec,
		DryRun:      false,
	}, slog.New(slog.DiscardHandler))

	stream.recvCh <- &protogen.ServerMessage{
		Message: &protogen.ServerMessage_Command{
			Command: &protogen.Command{
				Diid:    "diid-2",
				Name:    "kyverno",
				Version: "1.0.0",
				Chart:   "oci://example/kyverno",
			},
		},
	}
	close(stream.recvCh)

	_ = agent.Run(context.Background())

	if len(exec.deployReqs) != 1 {
		t.Fatalf("deployReqs = %d, want 1", len(exec.deployReqs))
	}
	if exec.deployReqs[0].Atomic {
		t.Errorf("regular feature deploy should not force Atomic=true")
	}
}

func TestAgentRefusesSelfUninstall(t *testing.T) {
	exec := &fakeExecutor{}
	stream := &fakeStream{recvCh: make(chan *protogen.ServerMessage, 1)}
	agent := NewAgent(&fakeClient{stream: stream}, AgentOptions{
		Tenant:      "nav",
		Environment: "dev",
		Executor:    exec,
		DryRun:      false,
	}, slog.New(slog.DiscardHandler))

	stream.recvCh <- &protogen.ServerMessage{
		Message: &protogen.ServerMessage_Command{
			Command: &protogen.Command{
				Diid:      "diid-3",
				Name:      "fasitd",
				Uninstall: true,
			},
		},
	}
	close(stream.recvCh)

	_ = agent.Run(context.Background())

	if len(exec.uninstallReq) != 0 {
		t.Errorf("uninstall should not be invoked for self")
	}
	statuses := stream.sentStatuses()
	if len(statuses) != 1 || statuses[0].GetStatus() != "failed" {
		t.Errorf("sent statuses = %v, want single 'failed'", statuses)
	}
}
