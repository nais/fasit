package testmanager

import (
	"context"
	"io/fs"
	"testing"
)

type CreateRunnerFunc func(ctx context.Context, config Config, state map[string]any) (runners []Runner, close func(), opts []Option, err error)

type Runner interface {
	Ext() string
	Run(ctx context.Context, logf func(format string, args ...any), body []byte, state map[string]any) error
}

type Manager struct {
	t              *testing.T
	createRunnerFn CreateRunnerFunc

	beforeHook Hook
}

type Option func(t *testCase)

func WithBeforeHook(hook Hook) Option {
	return func(t *testCase) {
		t.beforeHook = hook
	}
}

func New(t *testing.T, createRunners CreateRunnerFunc) *Manager {
	return &Manager{
		t:              t,
		createRunnerFn: createRunners,
	}
}

func (m *Manager) Run(ctx context.Context, dir fs.FS) error {
	entries, err := fs.ReadDir(dir, ".")
	if err != nil {
		m.t.Fatal("reading fs directory", err)
	}

	for _, d := range entries {
		if !d.IsDir() {
			continue
		}

		runTestCase(ctx, m, m.createRunnerFn, dir, d.Name())
	}

	return nil
}
