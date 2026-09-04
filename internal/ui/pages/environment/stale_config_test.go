package environment

import (
	"bytes"
	"strings"
	"testing"

	environmentpkg "github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
)

func TestStaleRowDeletesValueFromItsSource(t *testing.T) {
	t.Parallel()

	page := &FeaturePage{
		TenantSlug:  "tenant",
		Environment: &Environment{Environment: &environmentpkg.Environment{Name: "prod"}},
		Feature:     &FeatureDetail{Feature: &featurepkg.Feature{Name: "feature"}},
	}

	tests := []struct {
		name        string
		source      featurepkg.ConfigSource
		wantAction  string
		wantMessage string
	}{
		{
			name:        "environment value",
			source:      featurepkg.ConfigSourceEnv,
			wantAction:  "/features/feature/envs/tenant/prod/config/delete/00000000-0000-0000-0000-000000000001",
			wantMessage: "from this environment",
		},
		{
			name:        "global value",
			source:      featurepkg.ConfigSourceGlobal,
			wantAction:  "/features/feature/envs/tenant/prod/config/delete-global/00000000-0000-0000-0000-000000000001",
			wantMessage: "from all environments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := FeatureConfigItem{
				ID:     "00000000-0000-0000-0000-000000000001",
				Key:    "old-key",
				Source: string(tt.source),
			}

			var buf bytes.Buffer
			if err := staleRow(page, item).Render(&buf); err != nil {
				t.Fatalf("render stale row: %v", err)
			}
			html := buf.String()
			if !strings.Contains(html, `action="`+tt.wantAction+`"`) {
				t.Errorf("stale row action = %q, want %q", html, tt.wantAction)
			}
			if !strings.Contains(html, tt.wantMessage) {
				t.Errorf("stale row confirmation should say deletion is %s", tt.wantMessage)
			}
		})
	}
}
