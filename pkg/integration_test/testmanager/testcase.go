package testmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

type Hook func(ctx context.Context, config map[string]any)

type testCase struct {
	t        *testing.T
	dir      fs.FS
	runners  map[string]Runner
	state    map[string]any
	hasError bool
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

func runTestCase(ctx context.Context, t *testing.T, rfn CreateRunnerFunc, dir fs.FS, name string) {
	t.Run(name, func(t *testing.T) {
		config := map[string]any{}
		f, _ := fs.ReadFile(dir, filepath.Join(name, "00_config.json"))
		if f != nil {
			if err := json.Unmarshal(f, &config); err != nil {
				t.Fatalf("unmarshaling config: %v", err)
			}
		}

		tc := &testCase{
			t:     t,
			dir:   dir,
			state: map[string]any{},
		}

		runner, cleanup, err := rfn(ctx, config, tc.state)
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
