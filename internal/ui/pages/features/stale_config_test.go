package features

import (
	"bytes"
	"strings"
	"testing"

	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/ui/components"
)

func TestUnusedGlobalConfigIsClearlyRemovable(t *testing.T) {
	t.Parallel()

	item := components.ConfigItem{
		ID:     "00000000-0000-0000-0000-000000000001",
		Key:    "old-key",
		Value:  "old-value",
		Source: string(featurepkg.ConfigSourceGlobal),
	}

	var buf bytes.Buffer
	if err := orphanedConfigTable("feature", []components.ConfigItem{item}).Render(&buf); err != nil {
		t.Fatalf("render unused global config: %v", err)
	}
	html := buf.String()

	for _, want := range []string{
		"config-stale-section",
		"Unused configuration",
		">Delete<",
		`action="/features/feature/config/00000000-0000-0000-0000-000000000001/delete"`,
		"from all environments",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("unused global config should contain %q; got %q", want, html)
		}
	}
}
