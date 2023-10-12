package testmanager

import (
	"context"
	"io/fs"
	"slices"
	"testing"
)

type CreateRunnerFunc[T any] func(ctx context.Context, config T, state map[string]any) (runners []Runner, close func(), opts []Option, err error)

type Runner interface {
	Ext() string
	Run(ctx context.Context, logf func(format string, args ...any), body []byte, state map[string]any) error
}

type Manager[T any] struct {
	t              *testing.T
	createRunnerFn CreateRunnerFunc[T]

	beforeHook Hook
}

type Option func(t *testCase)

func WithBeforeHook(hook Hook) Option {
	return func(t *testCase) {
		t.beforeHook = hook
	}
}

func New[T any](t *testing.T, createRunners CreateRunnerFunc[T]) *Manager[T] {
	return &Manager[T]{
		t:              t,
		createRunnerFn: createRunners,
	}
}

func (m *Manager[T]) Run(ctx context.Context, dir fs.FS, skipDirs ...string) error {
	entries, err := fs.ReadDir(dir, ".")
	if err != nil {
		m.t.Fatal("reading fs directory", err)
	}

	for _, d := range entries {
		if !d.IsDir() {
			continue
		}

		if slices.Contains(skipDirs, d.Name()) {
			continue
		}

		runTestCase(ctx, m, m.createRunnerFn, dir, d.Name())
	}

	return nil
}
