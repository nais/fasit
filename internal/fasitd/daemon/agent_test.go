package daemon

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nais/fasit/internal/fasitd/commandstatus"
	"github.com/nais/fasit/internal/fasitd/protogen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"helm.sh/helm/v3/pkg/release"
)

const testGrace = 20 * time.Millisecond

type fakeStream struct {
	grpc.ClientStream
	ctx    context.Context
	recvCh chan *protogen.ServerMessage

	mu   sync.Mutex
	sent []*protogen.AgentMessage
}

func newFakeStream(buffer int) *fakeStream {
	return &fakeStream{recvCh: make(chan *protogen.ServerMessage, buffer)}
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

// statuses returns the reported terminal status per diid, in send order.
func (f *fakeStream) statuses() map[string]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]string{}
	for _, m := range f.sent {
		if s := m.GetStatus(); s != nil {
			out[s.GetDiid()] = s.GetStatus()
		}
	}
	return out
}

func (f *fakeStream) drainingCount() int {
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
	mu         sync.Mutex
	deployReqs []DeployRequest
	uninstalls []string
	deployFn   func(req DeployRequest) error
}

func (e *fakeExecutor) InstallOrUpgrade(_ context.Context, req DeployRequest, _ func(string)) error {
	e.mu.Lock()
	e.deployReqs = append(e.deployReqs, req)
	fn := e.deployFn
	e.mu.Unlock()
	if fn != nil {
		return fn(req)
	}
	return nil
}

func (e *fakeExecutor) Uninstall(_ context.Context, name string, _ time.Duration, _ func(string)) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.uninstalls = append(e.uninstalls, name)
	return nil
}

func newTestAgent(stream *fakeStream, opts AgentOptions) *Agent {
	opts.Tenant, opts.Environment = "nav", "dev"
	if opts.DrainGrace == 0 {
		opts.DrainGrace = testGrace
	}
	return NewAgent(&fakeClient{stream: stream}, opts, slog.New(slog.DiscardHandler))
}

func cmdMsg(c *protogen.Command) *protogen.ServerMessage {
	return &protogen.ServerMessage{Message: &protogen.ServerMessage_Command{Command: c}}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func TestAgentCommandOutcomes(t *testing.T) {
	tests := map[string]struct {
		dryRun       bool
		cmd          *protogen.Command
		wantStatus   commandstatus.Status
		wantDeploys  int
		wantAtomic   bool
		wantUninstll int
	}{
		"regular deploy is not atomic": {
			cmd:         &protogen.Command{Diid: "d", Name: "kyverno", Chart: "oci://x/kyverno", Version: "1"},
			wantStatus:  commandstatus.Deployed,
			wantDeploys: 1,
		},
		"uninstall reports uninstalled": {
			cmd:          &protogen.Command{Diid: "d", Name: "kyverno", Uninstall: true},
			wantStatus:   commandstatus.Uninstalled,
			wantUninstll: 1,
		},
		"self uninstall is refused": {
			cmd:        &protogen.Command{Diid: "d", Name: selfReleaseName, Uninstall: true},
			wantStatus: commandstatus.Failed,
		},
		"dry-run uninstall reports uninstalled without helm": {
			dryRun:     true,
			cmd:        &protogen.Command{Diid: "d", Name: "kyverno", Uninstall: true},
			wantStatus: commandstatus.Uninstalled,
		},
		"dry-run deploy reports deployed without helm": {
			dryRun:     true,
			cmd:        &protogen.Command{Diid: "d", Name: "kyverno", Version: "1"},
			wantStatus: commandstatus.Deployed,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			exec := &fakeExecutor{}
			stream := newFakeStream(1)
			agent := newTestAgent(stream, AgentOptions{Executor: exec, DryRun: tc.dryRun})

			stream.recvCh <- cmdMsg(tc.cmd)
			close(stream.recvCh)
			if err := agent.Run(context.Background()); err == nil {
				t.Fatalf("Run should return the recv error after the stream ends")
			}

			if got := stream.statuses()["d"]; got != string(tc.wantStatus) {
				t.Errorf("status = %q, want %q", got, tc.wantStatus)
			}
			if len(exec.deployReqs) != tc.wantDeploys {
				t.Fatalf("deploys = %d, want %d", len(exec.deployReqs), tc.wantDeploys)
			}
			if tc.wantDeploys > 0 && exec.deployReqs[0].Atomic != tc.wantAtomic {
				t.Errorf("Atomic = %v, want %v", exec.deployReqs[0].Atomic, tc.wantAtomic)
			}
			if len(exec.uninstalls) != tc.wantUninstll {
				t.Errorf("uninstalls = %d, want %d", len(exec.uninstalls), tc.wantUninstll)
			}
		})
	}
}

func TestAgentSelfUpgradeDrainsOnSIGTERMAndRunsAtomic(t *testing.T) {
	started := make(chan struct{})
	finish := make(chan struct{})
	exec := &fakeExecutor{deployFn: func(DeployRequest) error {
		close(started)
		<-finish
		return nil
	}}
	stream := newFakeStream(1)
	agent := newTestAgent(stream, AgentOptions{Executor: exec})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- agent.Run(ctx) }()
	waitFor(t, "registration", agent.Ready)

	stream.recvCh <- cmdMsg(&protogen.Command{Diid: "up", Name: selfReleaseName, Chart: "oci://x/fasitd", Version: "2"})
	<-started

	cancel()
	waitFor(t, "not ready after SIGTERM", func() bool { return !agent.Ready() })

	close(finish)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("Run did not exit after the in-flight command completed")
	}

	if !exec.deployReqs[0].Atomic {
		t.Errorf("self-upgrade must set Atomic=true")
	}
	if got := stream.statuses()["up"]; got != string(commandstatus.Deployed) {
		t.Errorf("self-upgrade status = %q, want deployed", got)
	}
	if got := stream.drainingCount(); got != 1 {
		t.Errorf("draining sent %d times, want 1", got)
	}
}

// Regression: a completion signal left over from an earlier command must not
// let shutdown cut the stream while a later command is still running.
func TestAgentShutdownWaitsForCurrentCommandAfterEarlierOnes(t *testing.T) {
	secondStarted := make(chan struct{})
	finishSecond := make(chan struct{})
	exec := &fakeExecutor{deployFn: func(req DeployRequest) error {
		if req.Name == "second" {
			close(secondStarted)
			<-finishSecond
		}
		return nil
	}}
	stream := newFakeStream(2)
	agent := newTestAgent(stream, AgentOptions{Executor: exec})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- agent.Run(ctx) }()

	stream.recvCh <- cmdMsg(&protogen.Command{Diid: "1", Name: "first"})
	stream.recvCh <- cmdMsg(&protogen.Command{Diid: "2", Name: "second"})
	<-secondStarted

	cancel()
	time.Sleep(5 * testGrace)
	close(finishSecond)
	<-done

	if got := stream.statuses()["2"]; got != string(commandstatus.Deployed) {
		t.Errorf("second command status = %q, want deployed (stream closed before it finished)", got)
	}
}

func TestAgentRejectsCommandsQueuedBehindSelfUpgrade(t *testing.T) {
	exec := &fakeExecutor{}
	stream := newFakeStream(2)
	agent := newTestAgent(stream, AgentOptions{Executor: exec})

	stream.recvCh <- cmdMsg(&protogen.Command{Diid: "up", Name: selfReleaseName})
	stream.recvCh <- cmdMsg(&protogen.Command{Diid: "late", Name: "kyverno"})

	if err := agent.Run(context.Background()); err != nil {
		t.Fatalf("Run after self-upgrade should return nil to reconnect, got %v", err)
	}

	got := stream.statuses()
	if got["up"] != string(commandstatus.Deployed) {
		t.Errorf("self-upgrade status = %q, want deployed", got["up"])
	}
	if got["late"] != string(commandstatus.Undeliverable) {
		t.Errorf("queued command status = %q, want undeliverable", got["late"])
	}
	if len(exec.deployReqs) != 1 {
		t.Errorf("deploys = %d, want only the self-upgrade", len(exec.deployReqs))
	}
}

type countingLister struct{ calls atomic.Int32 }

func (l *countingLister) List() ([]*release.Release, error) {
	l.calls.Add(1)
	return nil, nil
}

func TestAgentReleaseReportingStopsWithSession(t *testing.T) {
	lister := &countingLister{}
	stream := newFakeStream(0)
	agent := newTestAgent(stream, AgentOptions{ReleaseLister: lister, ReleaseInterval: time.Millisecond, DryRun: true})

	go func() {
		time.Sleep(10 * time.Millisecond)
		close(stream.recvCh)
	}()
	_ = agent.Run(context.Background())

	time.Sleep(10 * time.Millisecond)
	after := lister.calls.Load()
	time.Sleep(20 * time.Millisecond)
	if got := lister.calls.Load(); got != after {
		t.Errorf("release reporting kept running after the session ended (%d -> %d calls)", after, got)
	}
}
