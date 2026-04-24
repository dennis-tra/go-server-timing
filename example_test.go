package servertiming_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	servertiming "github.com/dennis-tra/go-server-timing"
)

func ExampleMiddleware() {
	handler := servertiming.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := servertiming.FromContext(r.Context())
		h.NewMetric("db").WithDuration(50 * time.Millisecond).WithDesc("select users")
		h.NewMetric("cache").WithDuration(2 * time.Millisecond)
		w.Write([]byte("ok"))
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	fmt.Println(rec.Result().Header.Get(servertiming.HeaderName))
	// Output: db;dur=50;desc="select users", cache;dur=2
}

func ExampleParseHeader() {
	h, err := servertiming.ParseHeader(`db;dur=53, cache;desc="Cache Read";dur=23.2`)
	if err != nil {
		fmt.Println("err:", err)
	}
	for _, m := range h.Metrics() {
		fmt.Printf("%s: dur=%v desc=%q\n", m.Name, m.Duration, m.Description)
	}
	// Output:
	// db: dur=53ms desc=""
	// cache: dur=23.2ms desc="Cache Read"
}

func ExampleMetric() {
	m := servertiming.NewMetric("db").
		WithDuration(123*time.Millisecond + 456*time.Microsecond).
		WithDesc("primary query")
	fmt.Println(m.String())
	// Output: db;dur=123.456;desc="primary query"
}

func ExampleHeader_startStop() {
	h := servertiming.NewHeader()
	func() {
		defer h.NewMetric("work").Start().Stop()
		// ... work happens here; Stop records elapsed time ...
	}()
	fmt.Println(len(h.Metrics()))
	// Output: 1
}

func ExampleParseHeaders() {
	// Parse multiple Server-Timing header values (common with proxies/middleware chains)
	values := []string{
		"db;dur=53;desc=\"Query\"",
		"cache;dur=12",
		"render;dur=89;desc=\"Template\"",
	}
	h, err := servertiming.ParseHeaders(values)
	if err != nil {
		fmt.Println("parse errors:", err)
	}
	for _, m := range h.Metrics() {
		fmt.Printf("%s: %v\n", m.Name, m.Duration)
	}
	// Output:
	// db: 53ms
	// cache: 12ms
	// render: 89ms
}

func ExampleMetric_WithParam() {
	// Add arbitrary key-value parameters to a metric
	m := servertiming.NewMetric("db").
		WithDuration(42*time.Millisecond).
		WithDesc("user lookup").
		WithParam("server", "primary-us-west").
		WithParam("version", "2")
	fmt.Println(m.String())
	// Output: db;dur=42;desc="user lookup";server=primary-us-west;version=2
}

func ExampleHeader_WriteTo() {
	// Manually construct and write metrics without middleware
	w := httptest.NewRecorder()
	h := servertiming.NewHeader()
	h.NewMetric("setup").WithDuration(10 * time.Millisecond)
	h.NewMetric("work").WithDuration(50 * time.Millisecond).WithDesc("main task")
	h.NewMetric("cleanup").WithDuration(5 * time.Millisecond)

	h.WriteTo(w.Header())
	fmt.Println(w.Header().Get(servertiming.HeaderName))
	// Output: setup;dur=10, work;dur=50;desc="main task", cleanup;dur=5
}

func ExampleMetric_String() {
	// Serialize a metric to its Server-Timing header representation
	m := servertiming.NewMetric("lookup").
		WithDuration(35*time.Millisecond).
		WithDesc("cache lookup").
		WithParam("cache", "redis")
	fmt.Println(m.String())
	// Output: lookup;dur=35;desc="cache lookup";cache=redis
}
