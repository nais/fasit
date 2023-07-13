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
	Logger        *logrus.Entry
	Timeout       time.Duration
	NumSuccessful *int
}

func (m *MockExecutor) Execute(cmd *exec.Cmd) error {
	m.Logger.Println(cmd.String())

	if cmd.Stdout != nil {
		fmt.Fprintln(cmd.Stdout, "Start mock executor", time.Now())
		start := time.Now()
		for time.Since(start) < 1*time.Minute {
			fmt.Fprintln(cmd.Stdout, "mock executor is running", time.Now())
			time.Sleep(5 * time.Second)
		}

		defer fmt.Fprintln(cmd.Stdout, "end of mock executor")
	}
	if m.Timeout > 0 {
		time.Sleep(m.Timeout)
	} else {
		time.Sleep(3 * time.Second)
	}

	var err error
	if m.NumSuccessful != nil {
		if *m.NumSuccessful <= 0 {
			err = fmt.Errorf("execution failed")
		}
		*m.NumSuccessful -= 1
	}

	return err
}

type Executor struct{}

func (m *Executor) Execute(cmd *exec.Cmd) error {
	return cmd.Run()
}
