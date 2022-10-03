package feature

import (
	"github.com/google/go-cmp/cmp"
	"testing"
)

func TestDependencies_FindMissing(t *testing.T) {
	tests := map[string]struct {
		dep      Dependencies
		features []string
		want     []string
	}{
		"empty": {
			dep:  Dependencies{},
			want: []string{},
		},
		"any of": {
			dep: Dependencies{
				{
					AnyOf: []string{"foo", "bar"},
				},
			},
			features: []string{"foo"},
			want:     []string{},
		},
		"all of": {
			dep: Dependencies{
				{
					AllOf: []string{"foo", "bar"},
				},
			},
			features: []string{"foo", "bar"},
			want:     []string{},
		},
		"all of and any of": {
			dep: Dependencies{
				{
					AllOf: []string{"foo", "bar"},
					AnyOf: []string{"baz", "qux"},
				},
			},
			features: []string{"foo", "bar", "baz"},
			want:     []string{},
		},
		"all of and any of, not satisfied": {
			dep: Dependencies{
				{
					AllOf: []string{"foo", "bar"},
					AnyOf: []string{"baz", "qux"},
				},
			},
			features: []string{"foo", "bar"},
			want:     []string{"baz", "qux"},
		},
		"all of, not satisfied": {
			dep: Dependencies{
				{
					AllOf: []string{"foo", "bar"},
				},
			},
			features: []string{"foo", "baz"},
			want:     []string{"bar"},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := tt.dep.FindMissing(tt.features); !cmp.Equal(tt.want, got) {
				t.Errorf("diff -want +got:\n%v", cmp.Diff(tt.want, got))
			}
		})
	}
}
