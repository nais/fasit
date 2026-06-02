package naisd

import (
	"io"
	"log/slog"
	"os/exec"
	"testing"
	"time"
)

func TestMockExecutor_Execute(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	m := &MockExecutor{
		Logger:  log,
		Timeout: 10 * time.Millisecond,
	}
	if err := m.Execute(exec.Command("ls")); err != nil {
		t.Errorf("MockExecutor.Execute() error = %v", err)
	}
}
