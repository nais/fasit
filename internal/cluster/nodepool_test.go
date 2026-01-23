package cluster

import "testing"

func TestContainsNodePool(t *testing.T) {
	tests := []struct {
		name         string
		targetLink   string
		nodepoolName string
		expected     bool
	}{
		{
			name:         "matches nodepool operation",
			targetLink:   "https://container.googleapis.com/v1/projects/my-project/locations/europe-north1/clusters/nais-dev/nodePools/gke-standard-postgres-pool",
			nodepoolName: "gke-standard-postgres-pool",
			expected:     true,
		},
		{
			name:         "does not match different nodepool",
			targetLink:   "https://container.googleapis.com/v1/projects/my-project/locations/europe-north1/clusters/nais-dev/nodePools/other-pool",
			nodepoolName: "gke-standard-postgres-pool",
			expected:     false,
		},
		{
			name:         "does not match when nodepool name is substring (pool vs pool-special)",
			targetLink:   "https://container.googleapis.com/v1/projects/my-project/locations/europe-north1/clusters/nais-dev/nodePools/pool-special",
			nodepoolName: "pool",
			expected:     false,
		},
		{
			name:         "does not match when nodepool name is superstring (pool-special vs pool)",
			targetLink:   "https://container.googleapis.com/v1/projects/my-project/locations/europe-north1/clusters/nais-dev/nodePools/pool",
			nodepoolName: "pool-special",
			expected:     false,
		},
		{
			name:         "handles URL with query parameters",
			targetLink:   "https://container.googleapis.com/v1/projects/my-project/locations/europe-north1/clusters/nais-dev/nodePools/my-pool?param=value",
			nodepoolName: "my-pool",
			expected:     true,
		},
		{
			name:         "handles URL with fragment",
			targetLink:   "https://container.googleapis.com/v1/projects/my-project/locations/europe-north1/clusters/nais-dev/nodePools/my-pool#fragment",
			nodepoolName: "my-pool",
			expected:     true,
		},
		{
			name:         "handles URL with both query and fragment",
			targetLink:   "https://container.googleapis.com/v1/projects/my-project/locations/europe-north1/clusters/nais-dev/nodePools/my-pool?param=value#fragment",
			nodepoolName: "my-pool",
			expected:     true,
		},
		{
			name:         "returns false for URL without nodePools segment",
			targetLink:   "https://container.googleapis.com/v1/projects/my-project/locations/europe-north1/clusters/nais-dev",
			nodepoolName: "my-pool",
			expected:     false,
		},
		{
			name:         "returns false for empty target link",
			targetLink:   "",
			nodepoolName: "my-pool",
			expected:     false,
		},
		{
			name:         "returns false for empty nodepool name",
			targetLink:   "https://container.googleapis.com/v1/projects/my-project/locations/europe-north1/clusters/nais-dev/nodePools/my-pool",
			nodepoolName: "",
			expected:     false,
		},
		{
			name:         "matches nodepool with hyphens and numbers",
			targetLink:   "https://container.googleapis.com/v1/projects/my-project/locations/europe-north1/clusters/nais-dev/nodePools/pool-123-special",
			nodepoolName: "pool-123-special",
			expected:     true,
		},
		{
			name:         "does not match when nodePools appears but no name follows",
			targetLink:   "https://container.googleapis.com/v1/projects/my-project/locations/europe-north1/clusters/nais-dev/nodePools",
			nodepoolName: "my-pool",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsNodePool(tt.targetLink, tt.nodepoolName)
			if result != tt.expected {
				t.Errorf("containsNodePool(%q, %q) = %v, want %v", tt.targetLink, tt.nodepoolName, result, tt.expected)
			}
		})
	}
}
