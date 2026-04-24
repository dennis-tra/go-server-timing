package servertiming

import (
	"bufio"
	"io"
	"net"
	"net/http"
)

// tw is the base Server-Timing response-writer wrapper. It embeds
// the underlying ResponseWriter and latches h.WriteTo(w.Header())
// into the response header map before the first Write or
// WriteHeader. tw does not on its own implement any of
// http.Flusher, http.Hijacker, http.Pusher, or io.ReaderFrom; those
// interfaces are exposed via the 16 permutation types defined below,
// selected by [wrap] to match the inner writer's capabilities.
type tw struct {
	http.ResponseWriter
	h           *Header
	wroteHeader bool
}

// ensureHeader writes the aggregated Server-Timing value into the
// response header map exactly once. It is safe to call repeatedly.
func (w *tw) ensureHeader() {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.h.WriteTo(w.ResponseWriter.Header())
}

func (w *tw) WriteHeader(code int) {
	w.ensureHeader()
	w.ResponseWriter.WriteHeader(code)
}

func (w *tw) Write(p []byte) (int, error) {
	w.ensureHeader()
	return w.ResponseWriter.Write(p)
}

// Unwrap returns the inner ResponseWriter so callers can use
// http.ResponseController (Go 1.20+) to reach through the wrapper.
func (w *tw) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// flush, hijack, push, readFrom are shared helpers for the
// permutation wrappers below. They centralize the ensureHeader
// timing decisions:
//
//   - Flush must emit the header first (buffered writers may send
//     bytes immediately after).
//   - Hijack leaves header handling to the caller; once the
//     connection is hijacked, the server library does not manage it.
//   - Push opens a new HTTP/2 stream; the current response is
//     independent and need not be finalized here.
//   - ReadFrom is a Write-path optimization, so the header must be
//     in place before bytes leave.
func (w *tw) flush() {
	w.ensureHeader()
	w.ResponseWriter.(http.Flusher).Flush()
}

func (w *tw) hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.ResponseWriter.(http.Hijacker).Hijack()
}

func (w *tw) push(target string, opts *http.PushOptions) error {
	return w.ResponseWriter.(http.Pusher).Push(target, opts)
}

func (w *tw) readFrom(src io.Reader) (int64, error) {
	w.ensureHeader()
	return w.ResponseWriter.(io.ReaderFrom).ReadFrom(src)
}

// wrap returns a ResponseWriter that implements exactly the subset
// of http.Flusher, http.Hijacker, http.Pusher, and io.ReaderFrom
// that the inner writer (accessible via base.ResponseWriter)
// implements. Type assertions against the returned value reflect
// the inner writer's real capabilities.
//
// The 16 permutation types below are mechanical; each embeds the
// base *tw and adds precisely the methods needed to satisfy its
// interface mix. The naming scheme uses the single-letter tags
// F=Flusher, H=Hijacker, P=Pusher, R=ReaderFrom in that fixed order.
func wrap(base *tw) http.ResponseWriter {
	inner := base.ResponseWriter
	_, fl := inner.(http.Flusher)
	_, hi := inner.(http.Hijacker)
	_, pu := inner.(http.Pusher)
	_, rf := inner.(io.ReaderFrom)

	bits := 0
	if fl {
		bits |= 1
	}
	if hi {
		bits |= 2
	}
	if pu {
		bits |= 4
	}
	if rf {
		bits |= 8
	}

	switch bits {
	case 0:
		return base
	case 1:
		return twF{base}
	case 2:
		return twH{base}
	case 3:
		return twFH{base}
	case 4:
		return twP{base}
	case 5:
		return twFP{base}
	case 6:
		return twHP{base}
	case 7:
		return twFHP{base}
	case 8:
		return twR{base}
	case 9:
		return twFR{base}
	case 10:
		return twHR{base}
	case 11:
		return twFHR{base}
	case 12:
		return twPR{base}
	case 13:
		return twFPR{base}
	case 14:
		return twHPR{base}
	case 15:
		return twFHPR{base}
	}
	return base
}

// ---- permutation wrappers (mechanical; do not embed in new types) ----

type twF struct{ *tw }

func (w twF) Flush() { w.tw.flush() }

type twH struct{ *tw }

func (w twH) Hijack() (net.Conn, *bufio.ReadWriter, error) { return w.tw.hijack() }

type twFH struct{ *tw }

func (w twFH) Flush()                                       { w.tw.flush() }
func (w twFH) Hijack() (net.Conn, *bufio.ReadWriter, error) { return w.tw.hijack() }

type twP struct{ *tw }

func (w twP) Push(t string, o *http.PushOptions) error { return w.tw.push(t, o) }

type twFP struct{ *tw }

func (w twFP) Flush()                                   { w.tw.flush() }
func (w twFP) Push(t string, o *http.PushOptions) error { return w.tw.push(t, o) }

type twHP struct{ *tw }

func (w twHP) Hijack() (net.Conn, *bufio.ReadWriter, error) { return w.tw.hijack() }
func (w twHP) Push(t string, o *http.PushOptions) error     { return w.tw.push(t, o) }

type twFHP struct{ *tw }

func (w twFHP) Flush()                                       { w.tw.flush() }
func (w twFHP) Hijack() (net.Conn, *bufio.ReadWriter, error) { return w.tw.hijack() }
func (w twFHP) Push(t string, o *http.PushOptions) error     { return w.tw.push(t, o) }

type twR struct{ *tw }

func (w twR) ReadFrom(src io.Reader) (int64, error) { return w.tw.readFrom(src) }

type twFR struct{ *tw }

func (w twFR) Flush()                                { w.tw.flush() }
func (w twFR) ReadFrom(src io.Reader) (int64, error) { return w.tw.readFrom(src) }

type twHR struct{ *tw }

func (w twHR) Hijack() (net.Conn, *bufio.ReadWriter, error) { return w.tw.hijack() }
func (w twHR) ReadFrom(src io.Reader) (int64, error)        { return w.tw.readFrom(src) }

type twFHR struct{ *tw }

func (w twFHR) Flush()                                       { w.tw.flush() }
func (w twFHR) Hijack() (net.Conn, *bufio.ReadWriter, error) { return w.tw.hijack() }
func (w twFHR) ReadFrom(src io.Reader) (int64, error)        { return w.tw.readFrom(src) }

type twPR struct{ *tw }

func (w twPR) Push(t string, o *http.PushOptions) error { return w.tw.push(t, o) }
func (w twPR) ReadFrom(src io.Reader) (int64, error)    { return w.tw.readFrom(src) }

type twFPR struct{ *tw }

func (w twFPR) Flush()                                   { w.tw.flush() }
func (w twFPR) Push(t string, o *http.PushOptions) error { return w.tw.push(t, o) }
func (w twFPR) ReadFrom(src io.Reader) (int64, error)    { return w.tw.readFrom(src) }

type twHPR struct{ *tw }

func (w twHPR) Hijack() (net.Conn, *bufio.ReadWriter, error) { return w.tw.hijack() }
func (w twHPR) Push(t string, o *http.PushOptions) error     { return w.tw.push(t, o) }
func (w twHPR) ReadFrom(src io.Reader) (int64, error)        { return w.tw.readFrom(src) }

type twFHPR struct{ *tw }

func (w twFHPR) Flush()                                       { w.tw.flush() }
func (w twFHPR) Hijack() (net.Conn, *bufio.ReadWriter, error) { return w.tw.hijack() }
func (w twFHPR) Push(t string, o *http.PushOptions) error     { return w.tw.push(t, o) }
func (w twFHPR) ReadFrom(src io.Reader) (int64, error)        { return w.tw.readFrom(src) }
