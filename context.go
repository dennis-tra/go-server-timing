package servertiming

import "context"

type ctxKey struct{}

// NewContext returns a copy of parent carrying h as the
// request-scoped Server-Timing collector. Handler code retrieves it
// with [FromContext].
func NewContext(parent context.Context, h *Header) context.Context {
	return context.WithValue(parent, ctxKey{}, h)
}

// FromContext returns the Header attached to ctx, or nil if none is
// installed. Methods on a nil *Header are no-ops, so callers can
// chain without nil-checking:
//
//	servertiming.FromContext(r.Context()).NewMetric("db").Start()
func FromContext(ctx context.Context) *Header {
	if ctx == nil {
		return nil
	}
	h, _ := ctx.Value(ctxKey{}).(*Header)
	return h
}
