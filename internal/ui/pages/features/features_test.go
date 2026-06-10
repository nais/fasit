package features

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/nais/fasit/internal/audit"
)

func TestFeatureIndexTableSourceKebabMenu(t *testing.T) {
	var buf bytes.Buffer
	err := featureIndexTable([]featureIndexRow{{
		Name:        "kyverno",
		Description: "policy engine",
		Source:      "https://github.com/nais/kyverno",
	}}).Render(&buf)
	if err != nil {
		t.Fatal(err)
	}

	html := buf.String()
	if !strings.Contains(html, `data-kebab-toggle="feature-kebab-0"`) {
		t.Fatalf("feature index should render kebab button for source, got %s", html)
	}
	if !strings.Contains(html, "View on GitHub") {
		t.Fatalf("feature index kebab should contain GitHub link, got %s", html)
	}
	if !strings.Contains(html, "https://github.com/nais/kyverno") {
		t.Fatalf("feature index kebab should link to source URL, got %s", html)
	}
	if strings.Count(html, `data-no-sort`) != 2 {
		t.Fatalf("description and kebab headers should be non-sortable, got %s", html)
	}
}

func TestDeploymentActorsByID(t *testing.T) {
	actors := assignmentActorsByID([]*audit.Entry{
		{
			Actor:      "tronghn@nais/fasit/123456789",
			Action:     audit.ActionCreated,
			ObjectType: audit.ObjectTypeFeatureAssignment,
			Metadata:   []byte(`{"deploymentId":"dep-1"}`),
		},
		{
			Actor:      "ignored@nais/fasit/1",
			Action:     audit.ActionUpdated,
			ObjectType: audit.ObjectTypeFeatureAssignment,
			Metadata:   []byte(`{"deploymentId":"dep-2"}`),
		},
	})

	if got := actors["dep-1"]; got != "tronghn@nais/fasit/123456789" {
		t.Fatalf("got %q, want deployment actor", got)
	}
	if _, ok := actors["dep-2"]; ok {
		t.Fatal("non-created deployment audit should not be used as deployment actor")
	}
}

func TestRecentDeploymentsRendersActor(t *testing.T) {
	var buf bytes.Buffer
	err := recentAssignments([]deployRow{{
		FeatureName: "kyverno",
		Version:     "2026-05-27-abc",
		Total:       1,
		Deployed:    1,
		When:        time.Now(),
		Actor:       "tronghn@nais/fasit/123456789",
	}}).Render(&buf)
	if err != nil {
		t.Fatal(err)
	}

	html := buf.String()
	if !strings.Contains(html, "@tronghn") {
		t.Fatalf("recent deployments should render actor name, got %s", html)
	}
	if !strings.Contains(html, "View workflow run") || !strings.Contains(html, "github.com/nais/fasit/actions/runs/123456789") {
		t.Fatalf("recent deployments should have workflow link in kebab menu, got %s", html)
	}
}

func TestRecentDeploymentsRollupStatus(t *testing.T) {
	var buf bytes.Buffer
	err := recentAssignments([]deployRow{{
		FeatureName: "kyverno",
		Version:     "2026-05-27-abc",
		Total:       4,
		Deployed:    2,
		Failed:      1,
		Pending:     1,
		When:        time.Now(),
	}}).Render(&buf)
	if err != nil {
		t.Fatal(err)
	}
	html := buf.String()
	if !strings.Contains(html, "2 deployed, 1 failed, 1 pending") {
		t.Fatalf("rollup should break down counts, got %s", html)
	}
	if !strings.Contains(html, "status-error") {
		t.Fatalf("rollup with a failure should use the error class, got %s", html)
	}
}

func TestRecentDeploymentsAllDeployed(t *testing.T) {
	var buf bytes.Buffer
	err := recentAssignments([]deployRow{{
		FeatureName: "kyverno",
		Version:     "2026-05-27-abc",
		Total:       3,
		Deployed:    3,
		When:        time.Now(),
	}}).Render(&buf)
	if err != nil {
		t.Fatal(err)
	}
	html := buf.String()
	if !strings.Contains(html, "Deployed") || !strings.Contains(html, "status-success") {
		t.Fatalf("all-deployed rollup should render a success status, got %s", html)
	}
}
