package plugins

import (
	"context"
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMemoryPluginConfigure(t *testing.T) {
	assert := assert.New(t)
	plugin := &MemoryPlugin{}

	err := plugin.Configure(context.Background(), map[string]interface{}{
		"cap": "6MB",
	})
	assert.Nil(err)
	prev := debug.SetGCPercent(100)
	assert.Equal(-1, prev)

	prevLimit := debug.SetMemoryLimit(0)
	assert.Equal(int64(6291456), prevLimit)
}

func TestMemoryPluginConfigureNothing(t *testing.T) {
	assert := assert.New(t)
	plugin := &MemoryPlugin{}

	err := plugin.Configure(context.Background(), map[string]interface{}{})
	assert.Nil(err)
	prev := debug.SetGCPercent(100)
	assert.NotEqual(-1, prev)

	prevLimit := debug.SetMemoryLimit(0)
	assert.NotEqual(int64(6291456), prevLimit)
}
