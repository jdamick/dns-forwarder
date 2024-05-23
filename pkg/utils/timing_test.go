package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSimpleTiming(t *testing.T) {
	assert := assert.New(t)

	timing := SimpleTiming("test")
	assert.NotNil(timing)
	dur := timing.Stop()
	assert.NotEqual(0, dur)
}

func TestSimpleScopeTiming(t *testing.T) {
	assert := assert.New(t)

	timing := SimpleScopeTiming("test")
	assert.NotNil(timing)
	timing()
}
