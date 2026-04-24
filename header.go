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
//
// Concurrency: [Header.Add] and [Header.NewMetric] are safe to call
// from any goroutine. However, [Metric] values are not themselves
// safe for concurrent mutation. If a handler fans out goroutines
// that mutate a Metric (for example via [Metric.Stop]), it is the
// caller's responsibility to ensure those mutations complete before
// the response is written or [Header.String]/[Header.WriteTo] is
// called. To reduce the race window, [Header.String] snapshots each
// Metric's primitive fields under Header's mutex before
// serialization.
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
// returns "". String snapshots each Metric under the Header mutex
// before formatting, so concurrent mutation of Metric fields during
// serialization cannot tear the slice; see the concurrency note on
// [Header] for the contract on Metric-level mutations.
func (h *Header) String() string {
	if h == nil {
		return ""
	}
	h.mu.Lock()
	if len(h.metrics) == 0 {
		h.mu.Unlock()
		return ""
	}
	snap := make([]Metric, len(h.metrics))
	for i, m := range h.metrics {
		snap[i] = *m
	}
	h.mu.Unlock()

	var b strings.Builder
	for i := range snap {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(snap[i].String())
	}
	return b.String()
}

// WriteTo appends the Server-Timing header value on hdr if h contains
// any metrics. Existing Server-Timing values on hdr (e.g. set by an
// upstream middleware or gateway) are preserved: the W3C spec
// treats multiple Server-Timing fields as one comma-joined list, so
// [http.Header.Add] is semantically correct here. A nil h, nil hdr,
// or empty collector is a no-op.
func (h *Header) WriteTo(hdr http.Header) {
	if h == nil || hdr == nil {
		return
	}
	value := h.String()
	if value == "" {
		return
	}
	hdr.Add(HeaderName, value)
}
