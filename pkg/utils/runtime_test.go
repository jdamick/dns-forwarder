package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMemoryFuncs(t *testing.T) {
	assert.NotEqual(t, 0, CurrentMemoryInUse())
	assert.NotEqual(t, 0, FreeMemory())
	assert.NotEqual(t, 0, TotalMemory())
}
