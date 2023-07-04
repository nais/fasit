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
	Logger  *logrus.Entry
	Timeout time.Duration
}

func (m *MockExecutor) Execute(cmd *exec.Cmd) error {
	m.Logger.Println(cmd.String())

	if cmd.Stdout != nil {
		fmt.Fprintln(cmd.Stdout, "Start mock executor", time.Now())
		defer fmt.Fprintln(cmd.Stdout, "end of mock executor")
	}
	if m.Timeout > 0 {
		time.Sleep(m.Timeout)
	} else {
		time.Sleep(3 * time.Second)
	}

	return nil
}

type Executor struct{}

func (m *Executor) Execute(cmd *exec.Cmd) error {
	return cmd.Run()
}
