package naisd

import (
	"fmt"
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

	fmt.Fprintln(cmd.Stdout, "Start mock executor", time.Now())
	time.Sleep(3 * time.Second)
	fmt.Fprintln(cmd.Stdout, "end of mock executor")

	return nil
}

type Executor struct{}

func (m *Executor) Execute(cmd *exec.Cmd) error {
	return cmd.Run()
}
