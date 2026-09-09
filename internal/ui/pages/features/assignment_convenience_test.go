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
		`Enter manually…`,
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

func TestNewFeatureAssignmentPopoverExplainsRegistryFailure(t *testing.T) {
	data := &DetailPage{
		CurrentFeature:          &featurepkg.Feature{Name: "naiserator"},
		AssignmentVersionsError: "Could not load versions from the chart registry. Enter a version manually.",
	}

	var buf bytes.Buffer
	if err := newFeatureAssignmentPopover(data).Render(&buf); err != nil {
		t.Fatalf("render popover: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "Could not load versions from the chart registry") {
		t.Errorf("popover should explain the registry failure: %s", html)
	}
	if !strings.Contains(html, `type="text" name="version"`) {
		t.Errorf("popover should offer a manual version input when the registry fails: %s", html)
	}
}

func TestVersionSelectWithoutVersionsRendersManualInput(t *testing.T) {
	var buf bytes.Buffer
	if err := versionSelect("set-version-version", "set-version-custom-version", "set-version-version-label", nil).Render(&buf); err != nil {
		t.Fatalf("render version select: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, `type="text" name="version"`) {
		t.Errorf("empty version list should render a manual version input: %s", html)
	}
	if strings.Contains(html, "<select") {
		t.Errorf("empty version list should not render a select: %s", html)
	}
}

func TestVersionSelectOmitsLoadAllWhenAllVersionsVisible(t *testing.T) {
	var buf bytes.Buffer
	if err := versionSelect("s", "c", "l", []string{"1.2.0", "1.1.0"}).Render(&buf); err != nil {
		t.Fatalf("render version select: %v", err)
	}
	if strings.Contains(buf.String(), "Load all versions") {
		t.Errorf("version select should not offer load-all when everything is visible: %s", buf.String())
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
			}, "naiserator", "oci://example.test/naiserator", assignmentSpecsViewPrefs(), "", nil)
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

func TestAssignmentCardSetVersionUsesVersionList(t *testing.T) {
	versions := []string{"1.11.0", "1.10.0", "1.9.0", "1.8.0", "1.7.0", "1.6.0", "1.5.0", "1.4.0", "1.3.0", "1.2.0", "1.1.0"}

	var buf bytes.Buffer
	if err := renderCard(card{
		Title:               "1.1.0",
		FeatureAssignmentID: "assignment-id",
	}, "naiserator", "oci://example.test/naiserator", assignmentSpecsViewPrefs(), "", versions).Render(&buf); err != nil {
		t.Fatalf("render card: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		`id="set-version-assignment-id"`,
		`name="version"`,
		`value="1.11.0">1.11.0`,
		`value="1.2.0">1.2.0`,
		`value="1.1.0" hidden="" data-extra-version="">1.1.0`,
		`Load all versions…`,
		`Enter manually…`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("set version popover HTML missing %q", want)
		}
	}
}

func TestVersionSelectLimitsVersionsAndOffersLoadAll(t *testing.T) {
	versions := []string{"1.12.0", "1.11.0", "1.10.0", "1.9.0", "1.8.0", "1.7.0", "1.6.0", "1.5.0", "1.4.0", "1.3.0", "1.2.0", "1.1.0"}

	var buf bytes.Buffer
	if err := versionSelect("set-version-version", "set-version-custom-version", "set-version-version-label", versions).Render(&buf); err != nil {
		t.Fatalf("render version select: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		`value="1.12.0">1.12.0`,
		`value="1.3.0">1.3.0`,
		`value="1.2.0" hidden="" data-extra-version="">1.2.0`,
		`value="1.1.0" hidden="" data-extra-version="">1.1.0`,
		`Load all versions…`,
		`Enter manually…`,
		`name="version_custom"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("version select HTML missing %q", want)
		}
	}
}
