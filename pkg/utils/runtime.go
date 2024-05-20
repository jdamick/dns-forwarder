package utils

import (
	"runtime"

	"github.com/mackerelio/go-osstat/memory"
	///"github.com/pbnjay/memory"
)

// Bytes in use
func CurrentMemoryInUse() uint64 {
	stats := runtime.MemStats{}
	runtime.ReadMemStats(&stats)
	return stats.HeapInuse
}

func FreeMemory() uint64 {
	s, err := memory.Get()
	if err != nil {
		return 0
	}
	return s.Total - s.Used
}

func TotalMemory() uint64 {
	s, err := memory.Get()
	if err != nil {
		return 0
	}
	return s.Total
}
