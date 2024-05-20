package utils

import (
	"time"

	"github.com/VictoriaMetrics/metrics"
	log "github.com/rs/zerolog/log"
)

func SimpleScopeTiming(label string) func() {
	timing := NewTimingStarted(label)
	return func() {
		timing.Stop()
	}
}

func SimpleTiming(label string) Timing {
	return NewTimingStarted(label)
}

type Timing interface {
	Start()
	Stop() time.Duration
}

type timing struct {
	label string
	start time.Time
}

func NewTimingStarted(label string) (t *timing) {
	return &timing{label: label, start: time.Now()}
}
func (t *timing) Start() {
	t.start = time.Now()
}
func (t *timing) Stop() time.Duration {
	d := time.Since(t.start)
	if t.label != "" {
		log.Debug().Msgf("[%s] time elapsed: %v", t.label, d)
		metrics.GetOrCreateHistogram(t.label).Update(float64(d.Milliseconds()))
	}
	return d
}
