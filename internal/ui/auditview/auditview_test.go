package auditview

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/audit"
)

func TestDisplayAction(t *testing.T) {
	tests := []struct {
		name   string
		entry  *audit.Entry
		expect string
	}{
		{
			name:   "regular action passes through",
			entry:  &audit.Entry{Action: audit.ActionCreated, ObjectType: audit.ObjectTypeFeature},
			expect: "created",
		},
		{
			name:   "triggered assignment maps to redeploy",
			entry:  &audit.Entry{Action: audit.ActionTriggered, ObjectType: audit.ObjectTypeFeatureAssignment},
			expect: "redeploy",
		},
		{
			name:   "redeploy action maps to redeploy",
			entry:  &audit.Entry{Action: audit.ActionRedeploy, ObjectType: audit.ObjectTypeFeatureAssignment},
			expect: "redeploy",
		},
		{
			name:   "triggered on non-assignment passes through",
			entry:  &audit.Entry{Action: audit.ActionTriggered, ObjectType: audit.ObjectTypeFeature},
			expect: "triggered",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DisplayAction(tt.entry)
			if got != tt.expect {
				t.Errorf("DisplayAction() = %q, want %q", got, tt.expect)
			}
		})
	}
}

func TestIsDescriptionRedundant(t *testing.T) {
	tests := []struct {
		name   string
		entry  *audit.Entry
		expect bool
	}{
		{
			name:   "configuration is redundant",
			entry:  &audit.Entry{ObjectType: audit.ObjectTypeConfiguration},
			expect: true,
		},
		{
			name:   "redeploy is redundant",
			entry:  &audit.Entry{Action: audit.ActionRedeploy, ObjectType: audit.ObjectTypeFeatureAssignment},
			expect: true,
		},
		{
			name:   "triggered assignment is redundant",
			entry:  &audit.Entry{Action: audit.ActionTriggered, ObjectType: audit.ObjectTypeFeatureAssignment},
			expect: true,
		},
		{
			name:   "created assignment is not redundant",
			entry:  &audit.Entry{Action: audit.ActionCreated, ObjectType: audit.ObjectTypeFeatureAssignment},
			expect: false,
		},
		{
			name:   "feature is not redundant",
			entry:  &audit.Entry{Action: audit.ActionUpdated, ObjectType: audit.ObjectTypeFeature},
			expect: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDescriptionRedundant(tt.entry)
			if got != tt.expect {
				t.Errorf("IsDescriptionRedundant() = %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestDescription(t *testing.T) {
	tests := []struct {
		name   string
		entry  *audit.Entry
		expect string
	}{
		{
			name: "assignment created with version and target metadata",
			entry: &audit.Entry{
				Action:      audit.ActionCreated,
				ObjectType:  audit.ObjectTypeFeatureAssignment,
				Description: "version 1.2.3 → nav/dev",
			},
			expect: "version 1.2.3 → nav/dev",
		},
		{
			name: "assignment created with version and label metadata",
			entry: &audit.Entry{
				Action:      audit.ActionCreated,
				ObjectType:  audit.ObjectTypeFeatureAssignment,
				Description: "version 2.0.0",
				Metadata:    json.RawMessage(`{"target":{"team":"nav","env":"dev"}}`),
			},
			expect: "version 2.0.0 → env=dev, team=nav",
		},
		{
			name: "non-empty description used directly",
			entry: &audit.Entry{
				Action:      audit.ActionUpdated,
				ObjectType:  audit.ObjectTypeFeature,
				Description: "changed chart URL",
			},
			expect: "changed chart URL",
		},
		{
			name: "empty description falls back to summary",
			entry: &audit.Entry{
				Action:     audit.ActionDeleted,
				ObjectType: audit.ObjectTypeConfiguration,
				ObjectID:   "myapp/port",
			},
			expect: "deleted config myapp/port",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Description(tt.entry)
			if got != tt.expect {
				t.Errorf("Description() = %q, want %q", got, tt.expect)
			}
		})
	}
}

func TestResourceHref(t *testing.T) {
	envID := uuid.New()
	tests := []struct {
		name   string
		entry  *audit.Entry
		expect string
	}{
		{
			name:   "feature links to feature page",
			entry:  &audit.Entry{ObjectType: audit.ObjectTypeFeature, ObjectID: "myapp"},
			expect: "/features/myapp",
		},
		{
			name:   "assignment with feature name links to feature",
			entry:  &audit.Entry{ObjectType: audit.ObjectTypeFeatureAssignment, ObjectID: "myapp"},
			expect: "/features/myapp",
		},
		{
			name:   "assignment with uuid returns empty",
			entry:  &audit.Entry{ObjectType: audit.ObjectTypeFeatureAssignment, ObjectID: uuid.New().String()},
			expect: "",
		},
		{
			name:   "assignment all returns empty",
			entry:  &audit.Entry{ObjectType: audit.ObjectTypeFeatureAssignment, ObjectID: "all"},
			expect: "",
		},
		{
			name:   "global config links to config tab with anchor",
			entry:  &audit.Entry{ObjectType: audit.ObjectTypeConfiguration, ObjectID: "myapp/port"},
			expect: "/features/myapp/config#config-port",
		},
		{
			name: "env config links to env config tab with anchor",
			entry: &audit.Entry{
				ObjectType:      audit.ObjectTypeConfiguration,
				ObjectID:        "myapp/port",
				EnvironmentID:   &envID,
				TenantName:      "nav",
				EnvironmentName: "dev",
			},
			expect: "/tenants/nav/envs/dev/features/myapp/config#config-port",
		},
		{
			name: "environment links to env page",
			entry: &audit.Entry{
				ObjectType:      audit.ObjectTypeEnvironment,
				TenantName:      "nav",
				EnvironmentName: "dev",
			},
			expect: "/tenants/nav/envs/dev",
		},
		{
			name:   "environment without tenant returns empty",
			entry:  &audit.Entry{ObjectType: audit.ObjectTypeEnvironment},
			expect: "",
		},
		{
			name:   "unknown type returns empty",
			entry:  &audit.Entry{ObjectType: "unknown", ObjectID: "x"},
			expect: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResourceHref(tt.entry)
			if got != tt.expect {
				t.Errorf("ResourceHref() = %q, want %q", got, tt.expect)
			}
		})
	}
}

func TestConfigChangeNode(t *testing.T) {
	tests := []struct {
		name       string
		entry      *audit.Entry
		expectNil  bool
		expectText string // substring check on rendered output
	}{
		{
			name:      "non-config returns nil",
			entry:     &audit.Entry{ObjectType: audit.ObjectTypeFeature},
			expectNil: true,
		},
		{
			name:      "no metadata returns nil",
			entry:     &audit.Entry{ObjectType: audit.ObjectTypeConfiguration},
			expectNil: true,
		},
		{
			name: "secret shows (secret)",
			entry: &audit.Entry{
				ObjectType: audit.ObjectTypeConfiguration,
				Metadata:   json.RawMessage(`{"secret":"true"}`),
			},
			expectText: "(secret)",
		},
		{
			name: "only new value shown for create",
			entry: &audit.Entry{
				ObjectType: audit.ObjectTypeConfiguration,
				Metadata:   json.RawMessage(`{"new":"\"hello\""}`),
			},
			expectText: "hello",
		},
		{
			name: "old and new shown with arrow",
			entry: &audit.Entry{
				ObjectType: audit.ObjectTypeConfiguration,
				Metadata:   json.RawMessage(`{"old":"\"foo\"","new":"\"bar\""}`),
			},
			expectText: "→",
		},
		{
			name: "deleted value shown with strikethrough class",
			entry: &audit.Entry{
				ObjectType: audit.ObjectTypeConfiguration,
				Metadata:   json.RawMessage(`{"old":"\"gone\""}`),
			},
			expectText: "gone",
		},
		{
			name: "long values truncated",
			entry: &audit.Entry{
				ObjectType: audit.ObjectTypeConfiguration,
				Metadata:   json.RawMessage(`{"new":"\"abcdefghijklmnopqrstuvwxyz0123456789\""}`),
			},
			expectText: "…",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := ConfigChangeNode(tt.entry)
			if tt.expectNil {
				if node != nil {
					t.Error("expected nil, got non-nil node")
				}
				return
			}
			if node == nil {
				t.Fatal("expected non-nil node, got nil")
			}
			// Render to string to check content
			var buf strings.Builder
			if err := node.Render(&buf); err != nil {
				t.Fatalf("render error: %v", err)
			}
			rendered := buf.String()
			if !strings.Contains(rendered, tt.expectText) {
				t.Errorf("rendered output %q does not contain %q", rendered, tt.expectText)
			}
		})
	}
}

func TestCleanVal(t *testing.T) {
	tests := []struct {
		input, expect string
	}{
		{`"hello"`, "hello"},
		{`hello`, "hello"},
		{`""`, ""},
		{`"has "quotes" inside"`, `has "quotes" inside`},
	}
	for _, tt := range tests {
		got := cleanVal(tt.input)
		if got != tt.expect {
			t.Errorf("cleanVal(%q) = %q, want %q", tt.input, got, tt.expect)
		}
	}
}

func TestTruncateVal(t *testing.T) {
	short := "short"
	if got := truncateVal(short); got != short {
		t.Errorf("truncateVal(%q) = %q, want %q", short, got, short)
	}

	long := "this string is definitely longer than twenty four characters"
	got := truncateVal(long)
	if len(got) > 24+len("…") {
		t.Errorf("truncateVal result too long: %q", got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("truncateVal(%q) should end with …, got %q", long, got)
	}
}
