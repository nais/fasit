package naisd

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
		Logger:  log.WithTime(now),
		Timeout: 10 * time.Millisecond,
	}
	if err := m.Execute(exec.Command("ls")); err != nil {
		t.Errorf("MockExecutor.Execute() error = %v", err)
	}
}
