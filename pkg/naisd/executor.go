package naisd

import (
	"os/exec"
	"time"

	"github.com/sirupsen/logrus"
)

type Exec interface {
	Execute(cmd *exec.Cmd) error
}

type MockExecutor struct {
	Logger *logrus.Entry
}

func (m *MockExecutor) Execute(cmd *exec.Cmd) error {
	m.Logger.Println(cmd.String())

	time.Sleep(3 * time.Second)
	return nil
}

type Executor struct{}

func (m *Executor) Execute(cmd *exec.Cmd) error {
	return cmd.Run()
}
