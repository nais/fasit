package featureutil

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSmartDotSplit(t *testing.T) {
	tests := map[string]struct {
		input    string
		expected []string
		err      error
	}{
		"empty": {
			input:    "",
			expected: []string{""},
		},
		"single_level": {
			input:    "test1",
			expected: []string{"test1"},
		},
		"multi_level": {
			input:    "test.a",
			expected: []string{"test", "a"},
		},
		"escaped dots": {
			input:    "test\\.a",
			expected: []string{"test.a"},
		},
		"end with .": {
			input: "test.a.",
			err:   errors.New("cannot end with `.`"),
		},
		"starts with .": {
			input: ".test.a",
			err:   errors.New("cannot start with `.`"),
		},
		"contains ..": {
			input: "test..a",
			err:   errors.New("invalid `.` on position 5"),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			retval, err := SmartDotSplit(tc.input)
			if err != nil {
				if tc.err == nil {
					t.Fatal(err)
				}
				if tc.err.Error() != err.Error() {
					t.Errorf("got %q, want %q", err, tc.err)
				}
			} else if tc.err != nil {
				t.Errorf("got nil, want %q", tc.err)
			}
			if !cmp.Equal(retval, tc.expected) {
				t.Error(cmp.Diff(retval, tc.expected))
			}
		})
	}
}
