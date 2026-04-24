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
