package workers

import (
	"github.com/sirupsen/logrus"
	"os/exec"
)

type executor interface {
	Execute(cmd *exec.Cmd) error
}

type MockExecutor struct {
	Logger *logrus.Entry
}

func (m *MockExecutor) Execute(cmd *exec.Cmd) error {
	m.Logger.Println(cmd.String())

	return nil
}

type Executor struct{}

func (m *Executor) Execute(cmd *exec.Cmd) error {
	return cmd.Run()
}
