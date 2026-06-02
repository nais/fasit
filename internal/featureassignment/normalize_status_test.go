package featureassignment

import "testing"

func TestNormalizeStatus(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"created", "PENDING"},
		{"CREATED", "PENDING"},
		{"Created", "PENDING"},
		{"invalidated", "PENDING"},
		{"INVALIDATED", "PENDING"},
		{"deployed", "DEPLOYED"},
		{"failed", "FAILED"},
		{"pending", "PENDING"},
		{"", "UNKNOWN"},
		{"something", "SOMETHING"},
	}
	for _, tt := range tests {
		if got := NormalizeStatus(tt.input); got != tt.want {
			t.Errorf("NormalizeStatus(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
