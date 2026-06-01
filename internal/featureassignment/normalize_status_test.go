package featureassignment

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeStatus(t *testing.T) {
	assert.Equal(t, "PENDING", NormalizeStatus("created"))
	assert.Equal(t, "PENDING", NormalizeStatus("CREATED"))
	assert.Equal(t, "PENDING", NormalizeStatus("Created"))
	assert.Equal(t, "PENDING", NormalizeStatus("invalidated"))
	assert.Equal(t, "PENDING", NormalizeStatus("INVALIDATED"))
	assert.Equal(t, "DEPLOYED", NormalizeStatus("deployed"))
	assert.Equal(t, "FAILED", NormalizeStatus("failed"))
	assert.Equal(t, "PENDING", NormalizeStatus("pending"))
	assert.Equal(t, "UNKNOWN", NormalizeStatus(""))
	assert.Equal(t, "SOMETHING", NormalizeStatus("something"))
}
