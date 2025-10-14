package internal

import (
	"github.com/mongodb/ftdc/events"
)

func (m *EventMetrics) Export() *events.Performance {
	out := &events.Performance{
		Timestamp: m.Time.AsTime(),
		ID:        m.Id,
		Timers: events.PerformanceTimers{
			Duration: m.Timers.Duration.AsDuration(),
			Total:    m.Timers.Total.AsDuration(),
		},
	}

	if m.Counters != nil {
		out.Counters.Number = m.Counters.Number
		out.Counters.Operations = m.Counters.Ops
		out.Counters.Size = m.Counters.Size
		out.Counters.Errors = m.Counters.Errors
	}

	if m.Gauges != nil {
		out.Gauges.State = m.Gauges.State
		out.Gauges.Workers = m.Gauges.Workers
		out.Gauges.Failed = m.Gauges.Failed
	}

	return out
}
