package environment

import (
	"bytes"
	"strings"
	"testing"
	"time"

	environment2 "github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/ui/breadcrumb"
)

func TestEnvironmentPageRendersSideMenuAndBreadcrumbs(t *testing.T) {
	var buf bytes.Buffer
	node := page(
		[]breadcrumb.Crumb{breadcrumb.Environments(), {Label: "dev-nais"}, breadcrumb.EnvironmentWithSwitcher("dev-nais", "dev", nil)},
		environmentTabFeatures,
		&environment2.Tenant{Name: "dev-nais"},
		&Environment{Environment: &environment2.Environment{Name: "dev"}},
		nil,
		nil,
		nil,
		"",
		"",
		[]environmentFeatureRow{{Name: "kyverno", Status: "DEPLOYED", Version: "1.2.3"}},
		[]releaseRow{{Release: &feature.Release{Name: "kyverno", Status: "deployed", Version: "1.2.3", Revision: 7}}},
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

func TestEnvironmentDetailsRendersNaisdHealthAsCallout(t *testing.T) {
	var buf bytes.Buffer
	node := environmentDetailsCard(
		&Environment{Environment: &environment2.Environment{Name: "dev"}},
		nil,
		"",
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

func TestEnvironmentPageShowsNaisdBootstrapUntilFirstReport(t *testing.T) {
	env := &Environment{Environment: &environment2.Environment{Name: "dev", Kind: environment2.EnvironmentKindTenant}}
	tenant := &environment2.Tenant{Name: "dev-nais"}

	render := func(health environmentHealth) string {
		var buf bytes.Buffer
		node := page(
			[]breadcrumb.Crumb{breadcrumb.Environments(), {Label: "dev-nais"}, breadcrumb.EnvironmentWithSwitcher("dev-nais", "dev", nil)},
			environmentTabFeatures,
			tenant,
			env,
			nil, nil, nil,
			"my-project-123",
			"",
			nil, nil,
			health,
		)
		if err := node.Render(&buf); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}

	html := render(environmentHealth{})
	for _, want := range []string{
		"Install naisd",
		"helm install naisd oci://europe-north1-docker.pkg.dev/nais-io/nais/feature/naisd",
		`--set tenantName=dev-nais`,
		`--set env=dev`,
		`--set envProjectId=my-project-123`,
		`--set deploySubscription=naisd-fasit-dev`,
		`--set management=false`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("bootstrap card should contain %q", want)
		}
	}

	html = render(environmentHealth{ReportedAt: time.Now(), HasReport: true})
	if strings.Contains(html, "Install naisd") {
		t.Errorf("bootstrap card should disappear once naisd has reported")
	}
}

func TestNaisdInstallCommandOnprem(t *testing.T) {
	env := &Environment{Environment: &environment2.Environment{Name: "onprem", Kind: environment2.EnvironmentKindOnprem}}
	cmd := naisdInstallCommand("nav", env, "")
	if !strings.Contains(cmd, `--set envProjectId=<gcp-project-id>`) {
		t.Errorf("missing project id should render a placeholder: %s", cmd)
	}
	if !strings.Contains(cmd, "google.useServiceAccountKey") {
		t.Errorf("onprem should include service account key flags: %s", cmd)
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
		{name: "stale", health: environmentHealth{ReportedAt: now.Add(-2 * time.Minute), HasReport: true}, class: "status-error", label: "stale"},
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
