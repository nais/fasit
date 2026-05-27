package environment

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/ui/breadcrumb"
)

func TestEnvironmentPageRendersSideMenuAndBreadcrumbs(t *testing.T) {
	var buf bytes.Buffer
	node := page(
		[]breadcrumb.Crumb{breadcrumb.Environments(), {Label: "dev-nais"}, breadcrumb.EnvironmentWithSwitcher("dev-nais", "dev", nil)},
		environmentTabFeatures,
		&model.Tenant{Name: "dev-nais"},
		&Environment{Environment: &model.Environment{Name: "dev"}},
		nil,
		nil,
		nil,
		"",
		[]environmentFeatureRow{{Name: "kyverno", Status: "DEPLOYED", Version: "1.2.3"}},
		[]*model.Release{{Name: "kyverno", Status: "deployed", Version: "1.2.3", Revision: 7}},
		environmentHealth{ReportedAt: time.Now(), HasReport: true},
	)
	if err := node.Render(&buf); err != nil {
		t.Fatal(err)
	}

	html := buf.String()
	if !strings.Contains(html, `href="/tenants/dev-nais/envs/dev?tab=features" class="active"`) {
		t.Fatalf("environment page should render the active side-menu item: %s", html)
	}
	if !strings.Contains(html, `href="/environments"`) {
		t.Fatalf("environment page should include breadcrumb back to environments: %s", html)
	}
	if !strings.Contains(html, `href="/features/kyverno/envs/dev-nais/dev"`) {
		t.Fatalf("feature row should link to canonical feature environment view: %s", html)
	}
	if !strings.Contains(html, "Features in this environment") {
		t.Fatalf("features tab should render feature table: %s", html)
	}
	if strings.Contains(html, "Actual state reported by naisd") {
		t.Fatalf("features page should not render helm releases content: %s", html)
	}
}

func TestEnvironmentOverviewRendersNaisdHealthAsCallout(t *testing.T) {
	var buf bytes.Buffer
	node := environmentOverviewCard(
		&Environment{Environment: &model.Environment{Name: "dev"}},
		nil,
		"",
		environmentHealth{ReportedAt: time.Now(), HasReport: true},
	)
	if err := node.Render(&buf); err != nil {
		t.Fatal(err)
	}

	html := buf.String()
	if !strings.Contains(html, "environment-health-item status-success") || !strings.Contains(html, "Naisd is healthy") {
		t.Fatalf("overview should render naisd health as a callout: %s", html)
	}
}

func TestNaisdHealthBucket(t *testing.T) {
	now := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		health environmentHealth
		class  string
		label  string
	}{
		{name: "no report", health: environmentHealth{}, class: "status-error", label: "no report"},
		{name: "healthy", health: environmentHealth{ReportedAt: now.Add(-30 * time.Second), HasReport: true}, class: "status-success", label: "healthy"},
		{name: "stale", health: environmentHealth{ReportedAt: now.Add(-2 * time.Minute), HasReport: true}, class: "status-pending", label: "stale"},
		{name: "dead", health: environmentHealth{ReportedAt: now.Add(-10 * time.Minute), HasReport: true}, class: "status-error", label: "dead"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			class, label := naisdHealthBucket(tt.health, now)
			if class != tt.class || label != tt.label {
				t.Fatalf("got %s/%s, want %s/%s", class, label, tt.class, tt.label)
			}
		})
	}
}
