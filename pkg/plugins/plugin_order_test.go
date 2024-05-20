package plugins

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPluginOrdering(t *testing.T) {

	assert := assert.New(t)

	pluginList := make([]Plugin, 0)
	pluginList = append(pluginList, &CachePlugin{})
	pluginList = append(pluginList, &MetricsPlugin{})

	assert.Equal(pluginList[0].Name(), "cache")
	assert.Equal(pluginList[1].Name(), "metrics")

	orderPlugins(pluginList)

	assert.Equal(pluginList[0].Name(), "metrics")
	assert.Equal(pluginList[1].Name(), "cache")
}
