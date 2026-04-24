// Package servertiming parses, constructs, and emits HTTP Server-Timing
// response headers as defined by https://w3c.github.io/server-timing/.
//
// The package provides:
//
//   - [Metric] and [Header] types for building and serializing metrics.
//   - [ParseHeader] and [ParseHeaders] for reading metrics from responses.
//   - Context helpers ([NewContext], [FromContext]) for propagating a
//     collector through request-scoped code.
//   - [Middleware] that injects a collector into request contexts and
//     writes the aggregated Server-Timing header to the response before
//     the body is flushed.
//
// The package has no dependencies outside the Go standard library.
package servertiming
