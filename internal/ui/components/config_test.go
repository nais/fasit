package components

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	g "maragu.dev/gomponents"
)

func renderNode(t *testing.T, node g.Node) string {
	t.Helper()
	var buf bytes.Buffer
	if err := node.Render(&buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func TestBulkConfigCell_NotSetVsEmptyString(t *testing.T) {
	tests := []struct {
		name      string
		item      ConfigItem
		want      string
		notWanted string
	}{
		{
			name:      "unset value renders not set",
			item:      ConfigItem{Key: "k", Type: "STRING", HasValue: false},
			want:      "&lt;not set&gt;",
			notWanted: "(empty string)",
		},
		{
			name:      "explicit empty string renders empty string label",
			item:      ConfigItem{Key: "k", Type: "STRING", HasValue: true, Value: ""},
			want:      "(empty string)",
			notWanted: "&lt;not set&gt;",
		},
		{
			name: "non-empty value renders the value",
			item: ConfigItem{Key: "k", Type: "STRING", HasValue: true, Value: "hello"},
			want: "hello",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			html := renderNode(t, BulkConfigCell("form", "", tc.item))
			if !strings.Contains(html, tc.want) {
				t.Errorf("want %q in:\n%s", tc.want, html)
			}
			if tc.notWanted != "" && strings.Contains(html, tc.notWanted) {
				t.Errorf("did not want %q in:\n%s", tc.notWanted, html)
			}
		})
	}
}

func TestComputedValueCell_NotSetVsEmptyString(t *testing.T) {
	notSet := renderNode(t, ComputedValueCell(ConfigItem{Key: "k", IsComputed: true, HasValue: false}))
	if !strings.Contains(notSet, "&lt;not set&gt;") {
		t.Errorf("unset computed should render &lt;not set&gt;, got:\n%s", notSet)
	}

	empty := renderNode(t, ComputedValueCell(ConfigItem{Key: "k", IsComputed: true, HasValue: true, Value: ""}))
	if !strings.Contains(empty, "(empty string)") {
		t.Errorf("empty-string computed should render (empty string), got:\n%s", empty)
	}
}

func TestConfigKeyCell_DisplayNameShowsKeyAsTitle(t *testing.T) {
	html := renderNode(t, ConfigKeyCell(ConfigItem{Key: "authSignin", DisplayName: "Auth service login URL"}))
	if !strings.Contains(html, "Auth service login URL") {
		t.Errorf("want display name rendered, got:\n%s", html)
	}
	if !strings.Contains(html, `title="authSignin"`) {
		t.Errorf("want key as title attribute, got:\n%s", html)
	}
}

func TestConfigEditPopover_HasFetchHookAndErrorSlot(t *testing.T) {
	item := ConfigItem{Key: "k", Type: "STRING", Value: "v", HasValue: true}
	html := renderNode(t, ConfigEditPopover("edit-k", "/features/f/config/1", "Edit", "Save", item))
	if !strings.Contains(html, "data-config-form") {
		t.Errorf("want data-config-form attribute on form, got:\n%s", html)
	}
	if !strings.Contains(html, "data-config-form-error") {
		t.Errorf("want inline error slot, got:\n%s", html)
	}
}

func TestConfigFormError_RendersHidden(t *testing.T) {
	html := renderNode(t, ConfigFormError())
	if !strings.Contains(html, "data-config-form-error") {
		t.Errorf("want data-config-form-error attribute, got:\n%s", html)
	}
	if !strings.Contains(html, "display:none") {
		t.Errorf("want error slot hidden by default, got:\n%s", html)
	}
}

func TestParseConfigValueAuto(t *testing.T) {
	t.Parallel()

	t.Run("valid JSON object is stored as JSON, not an escaped string", func(t *testing.T) {
		v, err := ParseConfigValueAuto(`{"a": 1}`, "STRING")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if string(raw) != `{"a":1}` {
			t.Errorf("stored %s, want real JSON object {\"a\":1}", raw)
		}
	})

	t.Run("valid JSON array is stored as an array", func(t *testing.T) {
		v, err := ParseConfigValueAuto(`["a", "b"]`, "STRING")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if string(raw) != `["a","b"]` {
			t.Errorf("stored %s, want [\"a\",\"b\"]", raw)
		}
	})

	t.Run("leading whitespace before JSON is still detected", func(t *testing.T) {
		v, err := ParseConfigValueAuto("\n  {\"a\":1}  ", "STRING")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		raw, _ := json.Marshal(v)
		if string(raw) != `{"a":1}` {
			t.Errorf("stored %s, want {\"a\":1}", raw)
		}
	})

	t.Run("malformed JSON-looking input errors", func(t *testing.T) {
		_, err := ParseConfigValueAuto(`{"a":}`, "STRING")
		if err == nil {
			t.Fatal("expected error for malformed JSON")
		}
		if !strings.Contains(err.Error(), "invalid JSON") {
			t.Errorf("error = %v, want to contain 'invalid JSON'", err)
		}
	})

	t.Run("plain string stays a raw string", func(t *testing.T) {
		v, err := ParseConfigValueAuto("just text", "STRING")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != "just text" {
			t.Errorf("got %v, want raw string", v)
		}
	})

	t.Run("STRING_ARRAY comma input still comma-splits", func(t *testing.T) {
		v, err := ParseConfigValueAuto("a, b ,c", "STRING_ARRAY")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, ok := v.([]string)
		if !ok {
			t.Fatalf("got type %T, want []string", v)
		}
		want := []string{"a", "b", "c"}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("INT is unchanged", func(t *testing.T) {
		v, err := ParseConfigValueAuto("42", "INT")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != 42 {
			t.Errorf("got %v, want 42", v)
		}
	})

	t.Run("BOOL is unchanged", func(t *testing.T) {
		v, err := ParseConfigValueAuto("true", "BOOL")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != true {
			t.Errorf("got %v, want true", v)
		}
	})
}
