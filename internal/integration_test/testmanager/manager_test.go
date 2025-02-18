package testmanager

import "testing"

func TestExt(t *testing.T) {
	tests := map[string]struct {
		input string
		want  string
	}{
		"empty": {
			input: "",
			want:  "",
		},

		"no ext": {
			input: "foo",
			want:  "",
		},

		"ext": {
			input: "foo.test",
			want:  "",
		},

		"multiple ext": {
			input: "foo.gql.test",
			want:  "gql.test",
		},

		"three ext": {
			input: "foo.another.gql.test",
			want:  "gql.test",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := ext(tc.input)
			if got != tc.want {
				t.Fatalf("ext(%q) = %q; want %q", tc.input, got, tc.want)
			}
		})
	}
}
