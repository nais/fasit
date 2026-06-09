package components

import "testing"

func TestColumnConsensus(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   []Emphasis
	}{
		{
			name:   "uniform column recedes entirely",
			values: []string{"0.0.16", "0.0.16", "0.0.16"},
			want:   []Emphasis{EmphasisConsensus, EmphasisConsensus, EmphasisConsensus},
		},
		{
			name:   "single deviation stands out",
			values: []string{"0.0.16", "0.0.15", "0.0.16"},
			want:   []Emphasis{EmphasisConsensus, EmphasisOutlier, EmphasisConsensus},
		},
		{
			name:   "minority group flagged",
			values: []string{"env config", "helm value", "helm value", "helm value"},
			want:   []Emphasis{EmphasisOutlier, EmphasisConsensus, EmphasisConsensus, EmphasisConsensus},
		},
		{
			name:   "tie for the top highlights nothing",
			values: []string{"a", "a", "b", "b"},
			want:   []Emphasis{EmphasisNone, EmphasisNone, EmphasisNone, EmphasisNone},
		},
		{
			name:   "single row has nothing to compare",
			values: []string{"only"},
			want:   []Emphasis{EmphasisNone},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ColumnConsensus(tt.values)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("row %d = %v, want %v (values=%v)", i, got[i], tt.want[i], tt.values)
				}
			}
		})
	}
}
