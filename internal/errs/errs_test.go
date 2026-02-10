package errs_test

import (
	"errors"
	"testing"

	"github.com/nais/fasit/internal/errs"
)

func TestErrors(t *testing.T) {
	tests := map[string]struct {
		err    error
		target error
		String string
	}{
		"ErrMissingRequiredFields": {
			err: &errs.ErrMissingRequiredFields{
				Fields: []string{"test"},
			},
			target: &errs.ErrMissingRequiredFields{},
			String: "missing required fields: [test]",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if !errors.Is(tc.err, tc.target) {
				t.Errorf("errors.Is(%T, %T) got false", tc.err, tc.target)
			}
			if tc.err.Error() != tc.String {
				t.Errorf("got %q, want %q", tc.err.Error(), tc.String)
			}
		})
	}
}
