package servertiming

import (
	"testing"
	"time"
)

func TestParseHeader_SpecExamples(t *testing.T) {
	// Examples drawn from https://w3c.github.io/server-timing/
	tests := []struct {
		name  string
		input string
		want  []*Metric
	}{
		{
			"miss db app",
			"miss, db;dur=53, app;dur=47.2",
			[]*Metric{
				{Name: "miss"},
				{Name: "db", Duration: 53 * time.Millisecond},
				{Name: "app", Duration: 47200 * time.Microsecond},
			},
		},
		{
			"customView dc desc",
			"customView, dc;desc=atl",
			[]*Metric{
				{Name: "customView"},
				{Name: "dc", Description: "atl"},
			},
		},
		{
			"cache quoted desc with dur",
			`cache;desc="Cache Read";dur=23.2`,
			[]*Metric{
				{Name: "cache", Duration: 23200 * time.Microsecond, Description: "Cache Read"},
			},
		},
		{
			"total dur then desc",
			`total;dur=123.4;desc="Total Time"`,
			[]*Metric{
				{Name: "total", Duration: 123400 * time.Microsecond, Description: "Total Time"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, err := ParseHeader(tt.input)
			if err != nil {
				t.Fatalf("ParseHeader(%q) returned error: %v", tt.input, err)
			}
			got := h.Metrics()
			if len(got) != len(tt.want) {
				t.Fatalf("got %d metrics, want %d: %+v", len(got), len(tt.want), got)
			}
			for i, m := range got {
				w := tt.want[i]
				if m.Name != w.Name {
					t.Errorf("[%d].Name = %q, want %q", i, m.Name, w.Name)
				}
				if m.Duration != w.Duration {
					t.Errorf("[%d].Duration = %v, want %v", i, m.Duration, w.Duration)
				}
				if m.Description != w.Description {
					t.Errorf("[%d].Description = %q, want %q", i, m.Description, w.Description)
				}
			}
		})
	}
}

func TestParseHeader_EmptyAndWhitespace(t *testing.T) {
	tests := []string{
		"",
		" ",
		",,,",
		" , , ",
	}
	for _, in := range tests {
		h, err := ParseHeader(in)
		if err != nil {
			t.Errorf("ParseHeader(%q) err = %v, want nil", in, err)
		}
		if len(h.Metrics()) != 0 {
			t.Errorf("ParseHeader(%q) metrics = %+v, want none", in, h.Metrics())
		}
	}
}

func TestParseHeader_CaseInsensitiveParams(t *testing.T) {
	h, err := ParseHeader("db;DUR=50;DESC=hello")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	m := h.Metrics()[0]
	if m.Duration != 50*time.Millisecond {
		t.Errorf("Duration = %v, want 50ms", m.Duration)
	}
	if m.Description != "hello" {
		t.Errorf("Description = %q, want hello", m.Description)
	}
}

func TestParseHeader_DuplicateParamLastWins(t *testing.T) {
	h, err := ParseHeader("db;dur=1;dur=2")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	m := h.Metrics()[0]
	if m.Duration != 2*time.Millisecond {
		t.Errorf("Duration = %v, want 2ms (last wins)", m.Duration)
	}
}

func TestParseHeader_InvalidDurKeptInExtra(t *testing.T) {
	h, err := ParseHeader(`db;dur=abc`)
	if err == nil {
		t.Fatal("want non-nil error for invalid dur")
	}
	m := h.Metrics()[0]
	if m.Duration != 0 {
		t.Errorf("Duration = %v, want 0", m.Duration)
	}
	if got := m.Extra["dur"]; got != "abc" {
		t.Errorf("Extra[dur] = %q, want abc", got)
	}
}

func TestParseHeader_UnknownParamsPreserved(t *testing.T) {
	h, err := ParseHeader(`db;dur=5;custom=value;flag`)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	m := h.Metrics()[0]
	if got := m.Extra["custom"]; got != "value" {
		t.Errorf("Extra[custom] = %q, want value", got)
	}
	if _, ok := m.Extra["flag"]; !ok {
		t.Errorf("Extra[flag] missing, want empty-string entry")
	}
}

func TestParseHeader_QuotedCommasAndSemicolons(t *testing.T) {
	in := `a;desc="x,y;z",b;dur=1`
	h, err := ParseHeader(in)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	ms := h.Metrics()
	if len(ms) != 2 {
		t.Fatalf("len = %d, want 2", len(ms))
	}
	if ms[0].Description != "x,y;z" {
		t.Errorf("ms[0].Description = %q, want x,y;z", ms[0].Description)
	}
	if ms[1].Duration != time.Millisecond {
		t.Errorf("ms[1].Duration = %v, want 1ms", ms[1].Duration)
	}
}

func TestParseHeader_EscapedQuotesInDesc(t *testing.T) {
	in := `db;desc="say \"hi\""`
	h, err := ParseHeader(in)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	m := h.Metrics()[0]
	if m.Description != `say "hi"` {
		t.Errorf("Description = %q, want %q", m.Description, `say "hi"`)
	}
}

func TestParseHeader_MalformedSalvagesRest(t *testing.T) {
	in := `bad name, good;dur=5`
	h, err := ParseHeader(in)
	if err == nil {
		t.Fatal("expected non-nil error for invalid metric name")
	}
	ms := h.Metrics()
	if len(ms) != 1 {
		t.Fatalf("len = %d, want 1 (salvaged)", len(ms))
	}
	if ms[0].Name != "good" || ms[0].Duration != 5*time.Millisecond {
		t.Errorf("salvaged = %+v, want good;dur=5ms", ms[0])
	}
}

func TestParseHeader_UnterminatedQuote(t *testing.T) {
	in := `db;desc="unterminated`
	h, err := ParseHeader(in)
	if err == nil {
		t.Error("expected non-nil error for unterminated quote")
	}
	// Metric with Name is still recorded, but desc assignment failed.
	ms := h.Metrics()
	if len(ms) != 1 || ms[0].Name != "db" {
		t.Errorf("metrics = %+v, want single db metric", ms)
	}
	if ms[0].Description != "" {
		t.Errorf("Description = %q, want empty (param rejected)", ms[0].Description)
	}
}

func TestParseHeader_EmptyValueAfterEquals(t *testing.T) {
	// "dur=" is malformed (empty token is not a valid value).
	h, err := ParseHeader("db;dur=")
	if err == nil {
		t.Error("expected error for db;dur=")
	}
	if len(h.Metrics()) != 1 {
		t.Errorf("expected metric salvaged, got %d", len(h.Metrics()))
	}
}

func TestParseHeaders_Multiple(t *testing.T) {
	h, err := ParseHeaders([]string{"db;dur=5", "cache;desc=miss, app;dur=10"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	ms := h.Metrics()
	if len(ms) != 3 {
		t.Fatalf("len = %d, want 3", len(ms))
	}
	names := []string{ms[0].Name, ms[1].Name, ms[2].Name}
	want := []string{"db", "cache", "app"}
	for i := range names {
		if names[i] != want[i] {
			t.Errorf("names[%d] = %q, want %q", i, names[i], want[i])
		}
	}
}

func TestParseHeader_RoundTrip(t *testing.T) {
	tests := []string{
		"db;dur=53",
		`cache;dur=23.2;desc="Cache Read"`,
		`app;desc="a,b;c=d";dur=10`,
		"m1, m2;dur=1, m3;desc=x",
		`db;dur=5;custom=value`,
		`db;desc="say \"hi\""`,
	}
	for _, in := range tests {
		t.Run(in, func(t *testing.T) {
			h, err := ParseHeader(in)
			if err != nil {
				t.Fatalf("first parse err = %v", err)
			}
			serialized := h.String()
			h2, err := ParseHeader(serialized)
			if err != nil {
				t.Fatalf("reparse %q err = %v", serialized, err)
			}
			if h.String() != h2.String() {
				t.Errorf("round-trip diverged:\n  first:  %q\n  second: %q", h.String(), h2.String())
			}
		})
	}
}

func FuzzParseHeader(f *testing.F) {
	seeds := []string{
		"",
		"db;dur=5",
		`cache;desc="Cache Read";dur=23.2`,
		"a, b, c;dur=1;desc=x",
		`db;desc="x,y;z",b`,
		`db;desc="say \"hi\""`,
		",,,",
		`"unterminated`,
		"bad name, good;dur=5",
		"db;desc=\"obs\xe9text\"",  // obs-text byte
		"db;desc=\"nul\x00byte\"",  // NUL in qdtext
		"db;desc=\"line\nfeed\"",   // LF in qdtext
		"db;desc=\"\xff\xfe\xfd\"", // high bytes run
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		h, _ := ParseHeader(s)
		_ = h.String()
	})
}
