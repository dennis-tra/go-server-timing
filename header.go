package servertiming

import (
	"net/http"
	"strings"
	"sync"
)

// HeaderName is the canonical HTTP header name for Server-Timing.
const HeaderName = "Server-Timing"

// Header is a concurrency-safe collector of Metric values bound to a
// single HTTP request. The zero value is ready to use. Methods on a
// nil *Header are no-ops, so handlers that retrieve the collector via
// [FromContext] without one installed can call [Header.Add] without
// nil-checking.
type Header struct {
	mu      sync.Mutex
	metrics []*Metric
}

// NewHeader returns an empty Header.
func NewHeader() *Header {
	return &Header{}
}

// Add records m on h and returns m so callers can continue building
// with [Metric.Start] or similar. Add is safe for concurrent use. If
// h is nil, Add returns m unchanged without recording it.
func (h *Header) Add(m *Metric) *Metric {
	if h == nil || m == nil {
		return m
	}
	h.mu.Lock()
	h.metrics = append(h.metrics, m)
	h.mu.Unlock()
	return m
}

// NewMetric creates a Metric with the given name, adds it to h, and
// returns it. On a nil h the metric is still returned, allowing
// fluent use in code paths that may not have a collector installed.
func (h *Header) NewMetric(name string) *Metric {
	return h.Add(NewMetric(name))
}

// Metrics returns a snapshot copy of the collected metrics. The
// returned slice is safe to mutate; the underlying Metric values are
// shared.
func (h *Header) Metrics() []*Metric {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]*Metric, len(h.metrics))
	copy(out, h.metrics)
	return out
}

// String serializes the collected metrics as a single Server-Timing
// header value with entries joined by ", ". An empty or nil Header
// returns "".
func (h *Header) String() string {
	if h == nil {
		return ""
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.metrics) == 0 {
		return ""
	}
	var b strings.Builder
	for i, m := range h.metrics {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(m.String())
	}
	return b.String()
}

// WriteTo sets the Server-Timing header on hdr if h contains any
// metrics. A nil h, nil hdr, or empty collector is a no-op.
func (h *Header) WriteTo(hdr http.Header) {
	if h == nil || hdr == nil {
		return
	}
	value := h.String()
	if value == "" {
		return
	}
	hdr.Set(HeaderName, value)
}
