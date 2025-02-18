package testmanager

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type Hook func(ctx context.Context)

type testCase struct {
	t        *testing.T
	dir      fs.FS
	runners  map[string]Runner
	state    map[string]any
	hasError bool

	beforeHook Hook
}

func (m *testCase) Register(runner Runner) error {
	if m.runners == nil {
		m.runners = make(map[string]Runner)
	}

	ext := runner.Ext() + ".test"
	if _, ok := m.runners[ext]; ok {
		return fmt.Errorf("runner for extension %q already registered", ext)
	}

	m.runners[ext] = runner

	return nil
}

func runTestCase[T any](ctx context.Context, m *Manager[T], rfn CreateRunnerFunc[T], dir fs.FS, name string) {
	m.t.Run(name, func(t *testing.T) {
		config := new(T)
		f, _ := fs.ReadFile(dir, filepath.Join(name, "00_config.yaml"))
		if f != nil {
			if err := yaml.Unmarshal(f, &config); err != nil {
				t.Fatalf("unmarshaling config: %v", err)
			}
		}

		tc := &testCase{
			t:          t,
			dir:        dir,
			state:      map[string]any{},
			beforeHook: m.beforeHook,
		}

		runner, cleanup, opts, err := rfn(ctx, *config, tc.state)
		if err != nil {
			t.Fatal(err)
		}
		for _, opt := range opts {
			opt(tc)
		}

		if err != nil {
			t.Fatalf("creating runner: %v", err)
		}
		if cleanup != nil {
			t.Cleanup(cleanup)
		}

		for _, r := range runner {
			if err := tc.Register(r); err != nil {
				t.Fatalf("registering runner: %v", err)
			}
		}

		entries, err := fs.ReadDir(dir, name)
		if err != nil {
			t.Errorf("reading fs directory: %v", err)
		}

		for _, f := range entries {
			if f.IsDir() {
				continue
			}

			tc.runTestFile(ctx, filepath.Join(name, f.Name()))
		}
	})
}

func (t *testCase) runTestFile(ctx context.Context, name string) {
	if !strings.HasSuffix(name, ".test") {
		return
	}

	t.t.Run(name, func(tt *testing.T) {
		if t.hasError {
			tt.Skip("previous test failed")
		}

		if t.beforeHook != nil {
			t.beforeHook(ctx)
		}

		runner, ok := t.runners[ext(name)]
		if !ok {
			t.hasError = true
			tt.Fatalf("no runner for extension %q", name)
		}

		body, err := fs.ReadFile(t.dir, name)
		if err != nil {
			t.hasError = true
			tt.Fatalf("reading file %q", name)
		}

		if err := runner.Run(ctx, tt.Logf, body, t.state); err != nil {
			t.hasError = true
			tt.Fatal(err)
		}
	})
}

func ext(name string) string {
	base := filepath.Base(name)

	parts := strings.Split(base, ".")
	if len(parts) < 3 {
		return ""
	}

	return strings.Join(parts[len(parts)-2:], ".")
}
