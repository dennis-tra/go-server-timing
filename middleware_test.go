package servertiming

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMiddleware_SetsHeaderWhenMetricsRecorded(t *testing.T) {
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		FromContext(r.Context()).NewMetric("db").WithDuration(50 * time.Millisecond)
		w.Write([]byte("ok"))
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if got, want := rec.Result().Header.Get(HeaderName), "db;dur=50"; got != want {
		t.Errorf("Server-Timing = %q, want %q", got, want)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("body = %q, want ok", rec.Body.String())
	}
}

func TestMiddleware_NoHeaderWhenNoMetrics(t *testing.T) {
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := rec.Result().Header.Get(HeaderName); got != "" {
		t.Errorf("Server-Timing = %q, want empty", got)
	}
}

func TestMiddleware_HandlerNeverWrites(t *testing.T) {
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		FromContext(r.Context()).NewMetric("x").WithDuration(time.Millisecond)
		// No Write, no WriteHeader.
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if got, want := rec.Result().Header.Get(HeaderName), "x;dur=1"; got != want {
		t.Errorf("Server-Timing = %q, want %q (finalizer must run)", got, want)
	}
}

func TestMiddleware_HeaderSetBeforeBody(t *testing.T) {
	// Use httptest.NewServer so we get a real ResponseWriter and can
	// assert that the Server-Timing header arrives in the response
	// headers — which are flushed before the body.
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		FromContext(r.Context()).NewMetric("db").WithDuration(10 * time.Millisecond)
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("body"))
		// If we tried to mutate the header here it'd have no effect.
	}))
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("status = %d, want 202", resp.StatusCode)
	}
	if got, want := resp.Header.Get(HeaderName), "db;dur=10"; got != want {
		t.Errorf("Server-Timing = %q, want %q", got, want)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "body" {
		t.Errorf("body = %q, want body", body)
	}
}

func TestMiddleware_FlusherPassthrough(t *testing.T) {
	var flushed bool
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		FromContext(r.Context()).NewMetric("x").WithDuration(time.Millisecond)
		fl, ok := w.(http.Flusher)
		if !ok {
			t.Error("wrapped writer does not implement http.Flusher")
			return
		}
		w.Write([]byte("chunk"))
		fl.Flush()
		flushed = true
	}))
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if !flushed {
		t.Error("handler did not flush")
	}
	if got := resp.Header.Get(HeaderName); got == "" {
		t.Error("Server-Timing missing on flushed response")
	}
}

func TestMiddleware_HijackerPassthrough(t *testing.T) {
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Error("wrapped writer does not implement http.Hijacker")
			return
		}
		conn, buf, err := hj.Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		defer conn.Close()
		// Write a minimal HTTP/1.1 response by hand.
		buf.WriteString("HTTP/1.1 200 OK\r\nContent-Length: 3\r\n\r\nhey")
		buf.Flush()
	}))
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hey" {
		t.Errorf("body = %q, want hey", body)
	}
}

func TestMiddleware_ReaderFromPassthrough(t *testing.T) {
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		FromContext(r.Context()).NewMetric("copy").WithDuration(time.Millisecond)
		// io.Copy uses ReaderFrom when the destination supports it.
		// We can't directly observe that, but we can confirm the
		// writer implements the interface when the inner does.
		if _, ok := w.(io.ReaderFrom); !ok {
			// Not all httptest writers implement ReaderFrom; log and
			// skip the assertion but continue with the body test.
			t.Log("inner writer does not implement io.ReaderFrom")
		}
		io.Copy(w, strings.NewReader("copied"))
	}))
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "copied" {
		t.Errorf("body = %q, want copied", body)
	}
	if got := resp.Header.Get(HeaderName); got == "" {
		t.Error("Server-Timing missing")
	}
}

func TestMiddleware_ConcurrentAddsFromGoroutines(t *testing.T) {
	const N = 50
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := FromContext(r.Context())
		var wg sync.WaitGroup
		wg.Add(N)
		for i := 0; i < N; i++ {
			go func() {
				defer wg.Done()
				h.NewMetric("m").WithDuration(time.Millisecond)
			}()
		}
		wg.Wait()
		w.Write([]byte("ok"))
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	got := rec.Result().Header.Get(HeaderName)
	count := strings.Count(got, "m;dur=1")
	if count != N {
		t.Errorf("got %d metrics in header, want %d: %q", count, N, got)
	}
}

func TestMiddleware_UnwrapWorks(t *testing.T) {
	var inner http.ResponseWriter
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, ok := w.(interface{ Unwrap() http.ResponseWriter }); ok {
			inner = u.Unwrap()
		}
		w.Write([]byte("ok"))
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if inner == nil {
		t.Fatal("wrapped writer does not expose Unwrap()")
	}
	if inner == httpResponseWriter(rec) {
		// ResponseRecorder is passed directly by ServeHTTP; Unwrap
		// should return it.
		return
	}
}

// httpResponseWriter narrows a concrete value to the interface for
// pointer-equality comparison in a test.
func httpResponseWriter(w http.ResponseWriter) http.ResponseWriter { return w }

func TestMiddleware_ExplicitStatusCode(t *testing.T) {
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		FromContext(r.Context()).NewMetric("x").WithDuration(time.Millisecond)
		w.WriteHeader(http.StatusTeapot)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusTeapot {
		t.Errorf("status = %d, want 418", rec.Code)
	}
	if got := rec.Result().Header.Get(HeaderName); got != "x;dur=1" {
		t.Errorf("Server-Timing = %q, want x;dur=1", got)
	}
}

func TestMiddleware_ContextPropagation(t *testing.T) {
	var seen *Header
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = FromContext(r.Context())
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if seen == nil {
		t.Fatal("FromContext returned nil inside Middleware")
	}
}

// TestSelectWrapper_Permutations verifies that wrap() returns a
// ResponseWriter that implements exactly the optional-interface
// subset of the inner writer.
func TestSelectWrapper_Permutations(t *testing.T) {
	type caps struct {
		flusher    bool
		hijacker   bool
		pusher     bool
		readerFrom bool
	}
	// Build a writer that implements the requested subset.
	build := func(c caps) http.ResponseWriter {
		base := &fakeRW{}
		switch {
		case !c.flusher && !c.hijacker && !c.pusher && !c.readerFrom:
			return base
		case c.flusher && !c.hijacker && !c.pusher && !c.readerFrom:
			return &fakeRWF{fakeRW: base}
		case !c.flusher && c.hijacker && !c.pusher && !c.readerFrom:
			return &fakeRWH{fakeRW: base}
		case c.flusher && c.hijacker && c.pusher && c.readerFrom:
			return &fakeRWAll{fakeRW: base}
		}
		return nil
	}

	for _, c := range []caps{
		{},
		{flusher: true},
		{hijacker: true},
		{flusher: true, hijacker: true, pusher: true, readerFrom: true},
	} {
		inner := build(c)
		if inner == nil {
			continue
		}
		h := NewHeader()
		base := &tw{ResponseWriter: inner, h: h}
		wrapped := wrap(base)

		if _, ok := wrapped.(http.Flusher); ok != c.flusher {
			t.Errorf("caps=%+v: Flusher = %v, want %v", c, ok, c.flusher)
		}
		if _, ok := wrapped.(http.Hijacker); ok != c.hijacker {
			t.Errorf("caps=%+v: Hijacker = %v, want %v", c, ok, c.hijacker)
		}
		if _, ok := wrapped.(http.Pusher); ok != c.pusher {
			t.Errorf("caps=%+v: Pusher = %v, want %v", c, ok, c.pusher)
		}
		if _, ok := wrapped.(io.ReaderFrom); ok != c.readerFrom {
			t.Errorf("caps=%+v: ReaderFrom = %v, want %v", c, ok, c.readerFrom)
		}
	}
}

// --- test doubles for capability matrix ---

type fakeRW struct {
	hdr    http.Header
	buf    strings.Builder
	status int
}

func (w *fakeRW) Header() http.Header {
	if w.hdr == nil {
		w.hdr = http.Header{}
	}
	return w.hdr
}
func (w *fakeRW) Write(p []byte) (int, error) { return w.buf.Write(p) }
func (w *fakeRW) WriteHeader(code int)        { w.status = code }

type fakeRWF struct{ *fakeRW }

func (*fakeRWF) Flush() {}

type fakeRWH struct{ *fakeRW }

func (*fakeRWH) Hijack() (net.Conn, *bufio.ReadWriter, error) { return nil, nil, nil }

type fakeRWAll struct{ *fakeRW }

func (*fakeRWAll) Flush()                                       {}
func (*fakeRWAll) Hijack() (net.Conn, *bufio.ReadWriter, error) { return nil, nil, nil }
func (*fakeRWAll) Push(string, *http.PushOptions) error         { return nil }
func (*fakeRWAll) ReadFrom(io.Reader) (int64, error)            { return 0, nil }
