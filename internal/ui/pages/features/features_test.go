package features

import "testing"

func TestComputeAggStatus(t *testing.T) {
	tests := []struct {
		name      string
		statuses  []string
		wantClass string
		wantLabel string
	}{
		{
			name:      "all deployed",
			statuses:  []string{"DEPLOYED", "DEPLOYED", "DEPLOYED"},
			wantClass: "status-success",
			wantLabel: "DEPLOYED",
		},
		{
			name:      "single deployed",
			statuses:  []string{"DEPLOYED"},
			wantClass: "status-success",
			wantLabel: "DEPLOYED",
		},
		{
			name:      "in progress no failures",
			statuses:  []string{"DEPLOYED", "DEPLOYED", "PENDING"},
			wantClass: "status-pending",
			wantLabel: "2/3 deployed",
		},
		{
			name:      "all pending",
			statuses:  []string{"PENDING", "PENDING", "PENDING"},
			wantClass: "status-pending",
			wantLabel: "0/3 deployed",
		},
		{
			name:      "mixed with failures",
			statuses:  []string{"DEPLOYED", "DEPLOYED", "FAILED", "PENDING"},
			wantClass: "status-error",
			wantLabel: "2 deployed, 1 failed, 1 pending",
		},
		{
			name:      "some failed rest deployed",
			statuses:  []string{"DEPLOYED", "DEPLOYED", "DEPLOYED", "FAILED"},
			wantClass: "status-error",
			wantLabel: "3 deployed, 1 failed",
		},
		{
			name:      "all failed",
			statuses:  []string{"FAILED", "FAILED", "FAILED"},
			wantClass: "status-error",
			wantLabel: "3 failed",
		},
		{
			name:      "pending install variant",
			statuses:  []string{"DEPLOYED", "PENDING-INSTALL"},
			wantClass: "status-pending",
			wantLabel: "1/2 deployed",
		},
		{
			name:      "unknown statuses counted",
			statuses:  []string{"DEPLOYED", "DEPLOYED", "SOMETHING_ELSE"},
			wantClass: "status-pending",
			wantLabel: "2/3 deployed",
		},
		{
			name:      "realistic: 15 deployed 2 failed 1 pending",
			statuses:  repeat("DEPLOYED", 15, "FAILED", 2, "PENDING", 1),
			wantClass: "status-error",
			wantLabel: "15 deployed, 2 failed, 1 pending",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeAggStatus(tt.statuses)
			if got.class != tt.wantClass {
				t.Errorf("class: got %q, want %q", got.class, tt.wantClass)
			}
			if got.label != tt.wantLabel {
				t.Errorf("label: got %q, want %q", got.label, tt.wantLabel)
			}
		})
	}
}

func repeat(args ...any) []string {
	var out []string
	for i := 0; i < len(args); i += 2 {
		s := args[i].(string)
		n := args[i+1].(int)
		for range n {
			out = append(out, s)
		}
	}
	return out
}
