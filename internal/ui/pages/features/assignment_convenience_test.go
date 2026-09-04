package features

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
)

func TestNewFeatureAssignmentPopoverUsesFeatureAndStructuredTargets(t *testing.T) {
	data := &DetailPage{
		CurrentFeature: &featurepkg.Feature{
			Name: "naiserator", Chart: "oci://example.test/charts/naiserator",
			FeatureYAML: featurepkg.FeatureYAML{
				EnvironmentKinds: []environment.EnvironmentKind{environment.EnvironmentKindTenant},
			},
		},
		AssignmentVersions: []string{"2.0.0", "1.0.0"},
		AssignmentLabelOptions: []assignmentLabelOption{
			{Key: "kind", Values: []string{"management", "tenant"}},
			{Key: "tenant", Values: []string{"dev-nais", "nav"}},
		},
	}

	var buf bytes.Buffer
	if err := newFeatureAssignmentPopover(data).Render(&buf); err != nil {
		t.Fatalf("render popover: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		`name="chart" value="oci://example.test/charts/naiserator"`,
		`data-version-select`,
		`Choose a version…`,
		`value="2.0.0">2.0.0`,
		`Enter another version…`,
		`name="environment_kind" value="tenant"`,
		`data-label-builder`,
		`data-label-key="kind"`,
		`data-label-key="tenant"`,
		`data-label-value="">nav`,
		`+ Add label`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("popover HTML missing %q", want)
		}
	}
	for _, unwanted := range []string{`target_labels_raw`, `Preview targets`, `assignment-version-options`, `same target will be replaced`} {
		if strings.Contains(html, unwanted) {
			t.Errorf("feature assignment popover should not contain %q", unwanted)
		}
	}
}

func TestAssignmentCardHighlightsCreatorsOutsideWorkflows(t *testing.T) {
	tests := []struct {
		name      string
		creator   string
		highlight bool
		want      string
	}{
		{name: "workflow", creator: "octocat@nais/fasit/123", want: "workflow"},
		{name: "person", creator: "user@example.com", highlight: true, want: "user@example.com"},
		{name: "unknown", highlight: true, want: "Unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			node := renderCard(card{
				Title:               "1.2.3",
				FeatureAssignmentID: "assignment-id",
				Creator:             tc.creator,
			}, "naiserator", "oci://example.test/naiserator", assignmentSpecsViewPrefs(), "")
			if err := node.Render(&buf); err != nil {
				t.Fatalf("render card: %v", err)
			}
			html := buf.String()
			if got := strings.Contains(html, "assignment-non-workflow"); got != tc.highlight {
				t.Errorf("highlight = %v, want %v; HTML: %s", got, tc.highlight, html)
			}
			if !strings.Contains(html, tc.want) {
				t.Errorf("card HTML missing %q", tc.want)
			}
		})
	}
}

func TestMergeVersionsKeepsRegistryOrderAndRemovesDuplicates(t *testing.T) {
	got := mergeVersions([]string{"3.0.0", "2.0.0"}, []string{"2.0.0", "1.0.0", ""})
	want := []string{"3.0.0", "2.0.0", "1.0.0"}
	if len(got) != len(want) {
		t.Fatalf("mergeVersions() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("mergeVersions()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
