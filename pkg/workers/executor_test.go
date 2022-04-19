package workers

import (
	"bytes"
	"os/exec"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestMockExecutor_Execute(t *testing.T) {
	log := logrus.New()
	buf := &bytes.Buffer{}
	log.SetOutput(buf)
	now := time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC)

	m := &MockExecutor{
		Logger: log.WithTime(now),
	}
	if err := m.Execute(exec.Command("ls")); err != nil {
		t.Errorf("MockExecutor.Execute() error = %v", err)
	}

	if buf.String() != "time=\"2022-01-01T00:00:00Z\" level=info msg=/usr/bin/ls\n" {
		t.Errorf("MockExecutor.Execute() output = %q", buf.String())
	}
}
