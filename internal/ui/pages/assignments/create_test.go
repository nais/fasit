package assignments

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewAssignmentDetailURL(t *testing.T) {
	id := uuid.MustParse("01965f3e-7b2a-7c3d-8e4f-5a6b7c8d9e0f")

	tests := []struct {
		name        string
		refererPath string
		want        string
	}{
		{
			name:        "assignment detail page",
			refererPath: "/features/myfeature/assignments/01965f3e-1111-7c3d-8e4f-5a6b7c8d9e0f",
			want:        "/features/myfeature/assignments/" + id.String(),
		},
		{
			name:        "assignments list page",
			refererPath: "/features/myfeature/assignments",
			want:        "",
		},
		{
			name:        "global assignments page",
			refererPath: "/assignments",
			want:        "",
		},
		{
			name:        "feature page",
			refererPath: "/features/myfeature",
			want:        "",
		},
		{
			name:        "non-uuid last segment",
			refererPath: "/features/myfeature/assignments/latest",
			want:        "",
		},
		{
			name:        "trailing slash is tolerated",
			refererPath: "/features/myfeature/assignments/01965f3e-1111-7c3d-8e4f-5a6b7c8d9e0f/",
			want:        "/features/myfeature/assignments/" + id.String(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := newAssignmentDetailURL(tc.refererPath, id); got != tc.want {
				t.Errorf("newAssignmentDetailURL(%q) = %q, want %q", tc.refererPath, got, tc.want)
			}
		})
	}
}
