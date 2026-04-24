package servertiming

import "net/http"

// Middleware wraps next so that a fresh [Header] collector is
// installed in the request context for the duration of the handler.
// Metrics recorded by the handler via [FromContext] are serialized
// into the Server-Timing response header before the first byte of
// the response body is written — or, if the handler never writes,
// before the net/http server finalizes the response.
//
// The wrapped ResponseWriter preserves the inner writer's optional
// interface implementations ([http.Flusher], [http.Hijacker],
// [http.Pusher], [io.ReaderFrom]) so downstream handlers that type
// assert against those interfaces continue to work.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := NewHeader()
		base := &tw{ResponseWriter: w, h: h}
		wrapped := wrap(base)
		ctx := NewContext(r.Context(), h)
		next.ServeHTTP(wrapped, r.WithContext(ctx))
		// Finalize in case the handler never wrote or explicitly
		// committed a status: this sets Server-Timing on the
		// response header map before the server flushes the default
		// response.
		base.ensureHeader()
	})
}
