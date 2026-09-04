package features

import (
	"bytes"
	"strings"
	"testing"
)

func TestAssignmentCardShowsDescription(t *testing.T) {
	cards := groupByAssignmentCards([]AssignmentEnvStatus{{
		FeatureAssignmentID:   "assignment-id",
		AssignmentVersion:     "1.2.3",
		AssignmentDescription: "Rollback to stable",
		StatusText:            "DEPLOYED",
	}}, "naiserator", nil)
	if len(cards) != 1 {
		t.Fatalf("got %d cards, want 1", len(cards))
	}

	var buf bytes.Buffer
	if err := renderCard(cards[0], "naiserator", "oci://example.test/naiserator", assignmentSpecsViewPrefs(), "").Render(&buf); err != nil {
		t.Fatalf("render card: %v", err)
	}
	html := buf.String()
	for _, want := range []string{`class="assignment-description"`, `title="Rollback to stable"`, `>Rollback to stable</span>`} {
		if !strings.Contains(html, want) {
			t.Errorf("assignment card missing %q: %s", want, html)
		}
	}
}

func TestAssignmentCardOmitsInternalDefaultDescription(t *testing.T) {
	var buf bytes.Buffer
	if err := renderCard(card{
		Title:       "1.2.3",
		Description: "Set via UI",
		Environments: []AssignmentEnvStatus{{
			StatusText: "DEPLOYED",
		}},
	}, "naiserator", "oci://example.test/naiserator", assignmentSpecsViewPrefs(), "").Render(&buf); err != nil {
		t.Fatalf("render card: %v", err)
	}
	if strings.Contains(buf.String(), "Set via UI") {
		t.Errorf("assignment card exposes internal default description: %s", buf.String())
	}
}
