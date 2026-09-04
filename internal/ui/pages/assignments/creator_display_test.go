package assignments

import (
	"bytes"
	"strings"
	"testing"
)

func TestAssignmentsTableShowsAndHighlightsCreator(t *testing.T) {
	rows := []Summary{
		{FeatureName: "workflow-feature", Version: "1.0.0", FeatureAssignmentID: "workflow-id", Creator: "octocat@nais/repo/123", Active: true},
		{FeatureName: "person-feature", Version: "2.0.0", FeatureAssignmentID: "person-id", Creator: "user@example.com", Active: true},
	}

	var buf bytes.Buffer
	if err := assignmentsTable(rows).Render(&buf); err != nil {
		t.Fatalf("render assignments table: %v", err)
	}
	html := buf.String()
	for _, want := range []string{"Created by", "workflow", "user@example.com"} {
		if !strings.Contains(html, want) {
			t.Errorf("table HTML missing %q", want)
		}
	}
	if got := strings.Count(html, "assignment-non-workflow"); got != 1 {
		t.Errorf("non-workflow highlight count = %d, want 1", got)
	}
}
