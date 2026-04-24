package servertiming

import (
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestHeader_Nil(t *testing.T) {
	var h *Header
	if m := h.Add(NewMetric("db")); m == nil || m.Name != "db" {
		t.Errorf("Add on nil returned %+v, want metric with Name=db", m)
	}
	if m := h.NewMetric("cache"); m == nil {
		t.Error("NewMetric on nil returned nil")
	}
	if got := h.Metrics(); got != nil {
		t.Errorf("Metrics on nil = %v, want nil", got)
	}
	if got := h.String(); got != "" {
		t.Errorf("String on nil = %q, want empty", got)
	}
	h.WriteTo(http.Header{})
	h.WriteTo(nil)
}

func TestHeader_Add(t *testing.T) {
	h := NewHeader()
	h.Add(NewMetric("db"))
	h.Add(NewMetric("cache"))
	h.Add(nil) // no-op
	if n := len(h.Metrics()); n != 2 {
		t.Errorf("len(Metrics) = %d, want 2", n)
	}
}

func TestHeader_String(t *testing.T) {
	h := NewHeader()
	h.Add(NewMetric("db").WithDuration(50 * time.Millisecond))
	h.Add(NewMetric("cache").WithDesc("miss"))
	const want = "db;dur=50, cache;desc=miss"
	if got := h.String(); got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
}

func TestHeader_StringEmpty(t *testing.T) {
	h := NewHeader()
	if got := h.String(); got != "" {
		t.Errorf("String on empty = %q, want empty", got)
	}
}

func TestHeader_ConcurrentAdd(t *testing.T) {
	h := NewHeader()
	const N = 100
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			h.Add(NewMetric("m"))
		}()
	}
	wg.Wait()
	if got := len(h.Metrics()); got != N {
		t.Errorf("len(Metrics) = %d, want %d", got, N)
	}
}

func TestHeader_WriteTo(t *testing.T) {
	h := NewHeader()
	h.Add(NewMetric("db").WithDuration(10 * time.Millisecond))
	hdr := http.Header{}
	h.WriteTo(hdr)
	if got, want := hdr.Get(HeaderName), "db;dur=10"; got != want {
		t.Errorf("Header = %q, want %q", got, want)
	}
}

func TestHeader_WriteToEmpty(t *testing.T) {
	h := NewHeader()
	hdr := http.Header{}
	h.WriteTo(hdr)
	if got := hdr.Get(HeaderName); got != "" {
		t.Errorf("Header = %q, want empty (no metrics)", got)
	}
}

func TestHeader_MetricsSnapshotIsolated(t *testing.T) {
	h := NewHeader()
	h.Add(NewMetric("db"))
	snapshot := h.Metrics()
	h.Add(NewMetric("cache"))
	if len(snapshot) != 1 {
		t.Errorf("snapshot mutated: len = %d, want 1", len(snapshot))
	}
}

func TestHeader_NewMetricIsAdded(t *testing.T) {
	h := NewHeader()
	m := h.NewMetric("db").WithDuration(5 * time.Millisecond)
	if m.Name != "db" {
		t.Errorf("NewMetric.Name = %q, want db", m.Name)
	}
	if got := len(h.Metrics()); got != 1 {
		t.Errorf("len(Metrics) = %d, want 1", got)
	}
}
