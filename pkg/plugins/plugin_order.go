package plugins

import (
	"slices"
)

var (
	// this is the plugin processing order for queries.
	// responses will follow this order in reverse.
	pluginOrder = []string{
		"metrics",
		"dns",
		"http",
		"https",
		"doq",
		"querylogger",
		"cache",
		"dnsclient",
	}
	pluginOrderMap = map[string]int{}
)

func init() {
	for i, name := range pluginOrder {
		pluginOrderMap[name] = i
	}
}

func orderPlugins[P Plugin](plugins []P) {
	pluginLen := len(plugins)
	slices.SortFunc(plugins, func(a, b P) int {
		aIdx, ok := pluginOrderMap[a.Name()]
		if !ok {
			aIdx = pluginLen
		}
		bIdx, ok := pluginOrderMap[b.Name()]
		if !ok {
			bIdx = pluginLen
		}
		if aIdx < bIdx {
			return -1
		} else if aIdx > bIdx {
			return 1
		}
		return 0
	})
}
