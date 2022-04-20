package graph

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
)

func TestMarshalUUID(t *testing.T) {
	tests := map[string]struct {
		input    uuid.UUID
		expected string
	}{
		"nil": {
			input:    uuid.Nil,
			expected: "null",
		},
		"non-nil": {
			input:    uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"),
			expected: "\"a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11\"",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			buf := bytes.Buffer{}
			m := MarshalUUID(tc.input)
			m.MarshalGQL(&buf)
			if buf.String() != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, buf.String())
			}
		})
	}
}

func TestUnmarshalUUID(t *testing.T) {
	tests := map[string]struct {
		input       any
		expected    uuid.UUID
		shouldError bool
	}{
		"non-nil": {
			input:       "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11",
			expected:    uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"),
			shouldError: false,
		},
		"non-uuid": {
			input:       "not-a-uuid",
			expected:    uuid.Nil,
			shouldError: true,
		},
		"non-string": {
			input:       123,
			expected:    uuid.Nil,
			shouldError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			u, err := UnmarshalUUID(tc.input)
			if (err != nil) != tc.shouldError {
				t.Errorf("expected error %v, got %v", tc.shouldError, err)
			}
			if u != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, u)
			}
		})
	}
}

func TestMarshalRawMessage(t *testing.T) {
	tests := map[string]struct {
		input    json.RawMessage
		expected string
	}{
		"nil": {
			input:    []byte("null"),
			expected: "null",
		},
		"non-nil": {
			input:    []byte(`"foo"`),
			expected: `"foo"`,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			buf := bytes.Buffer{}
			m := MarshalRawMessage(tc.input)
			m.MarshalGQL(&buf)
			if buf.String() != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, buf.String())
			}
		})
	}
}

func TestUnmarshalRawMessage(t *testing.T) {
	tests := map[string]struct {
		input       any
		expected    json.RawMessage
		shouldError bool
	}{
		"non-nil": {
			input:       "foo",
			expected:    []byte(`"foo"`),
			shouldError: false,
		},
		"non-string": {
			input:       123,
			expected:    []byte("123"),
			shouldError: false,
		},
		"struct": {
			input:       struct{ A int }{A: 123},
			expected:    []byte("{\"A\":123}"),
			shouldError: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			u, err := UnmarshalRawMessage(tc.input)
			if (err != nil) != tc.shouldError {
				t.Errorf("expected error %v, got %v", tc.shouldError, err)
			}
			if !cmp.Equal(u, tc.expected) {
				t.Error(cmp.Diff(u, tc.expected))
			}
		})
	}
}
