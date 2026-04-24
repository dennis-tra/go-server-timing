package servertiming

import (
	"testing"
	"testing/synctest"
	"time"
)

func TestMetric_String(t *testing.T) {
	tests := []struct {
		name string
		m    *Metric
		want string
	}{
		{"name only", NewMetric("db"), "db"},
		{"integer ms", NewMetric("db").WithDuration(123 * time.Millisecond), "db;dur=123"},
		{"fractional ms", NewMetric("db").WithDuration(123456 * time.Microsecond), "db;dur=123.456"},
		{"sub-ms", NewMetric("db").WithDuration(500 * time.Microsecond), "db;dur=0.5"},
		{"zero duration omitted", NewMetric("db").WithDuration(0), "db"},
		{"desc token", NewMetric("db").WithDesc("query"), "db;desc=query"},
		{"desc with space quoted", NewMetric("db").WithDesc("hello world"), `db;desc="hello world"`},
		{"desc with quote escaped", NewMetric("db").WithDesc(`a"b`), `db;desc="a\"b"`},
		{"desc with backslash escaped", NewMetric("db").WithDesc(`a\b`), `db;desc="a\\b"`},
		{"desc with comma quoted", NewMetric("db").WithDesc("a,b"), `db;desc="a,b"`},
		{"desc with semicolon quoted", NewMetric("db").WithDesc("a;b"), `db;desc="a;b"`},
		{"empty desc omitted", NewMetric("db").WithDesc(""), "db"},
		{"duration and desc", NewMetric("db").WithDuration(50 * time.Millisecond).WithDesc("query"), "db;dur=50;desc=query"},
		{"extra sorted alpha", NewMetric("db").WithParam("zzz", "1").WithParam("aaa", "2"), "db;aaa=2;zzz=1"},
		{"extra value with comma quoted", NewMetric("db").WithParam("x", "a,b"), `db;x="a,b"`},
		{"extra empty value quoted", NewMetric("db").WithParam("x", ""), `db;x=""`},
		{"dur via WithParam", NewMetric("db").WithParam("dur", "42.5"), "db;dur=42.5"},
		{"desc via WithParam", NewMetric("db").WithParam("desc", "query"), "db;desc=query"},
		{"invalid dur via WithParam kept raw", NewMetric("db").WithParam("dur", "NaN?"), `db;dur="NaN?"`},
		{"uppercase param name lowercased", NewMetric("db").WithParam("FOO", "bar"), "db;foo=bar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.m.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMetric_StartStop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m := NewMetric("db").Start()
		time.Sleep(2 * time.Millisecond)
		m.Stop()
		if m.Duration != 2*time.Millisecond {
			t.Errorf("Duration = %v, want 2ms exactly (synctest)", m.Duration)
		}
	})
}

func TestMetric_StopWithoutStart(t *testing.T) {
	m := NewMetric("db").WithDuration(5 * time.Millisecond)
	m.Stop()
	if m.Duration != 5*time.Millisecond {
		t.Errorf("Duration = %v, want unchanged 5ms", m.Duration)
	}
}

func TestMetric_WithParamOverwrites(t *testing.T) {
	m := NewMetric("db").WithParam("foo", "a").WithParam("foo", "b")
	if got := m.Extra["foo"]; got != "b" {
		t.Errorf("Extra[foo] = %q, want %q (last-wins)", got, "b")
	}
}

func TestIsToken(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"", false},
		{"db", true},
		{"db_query", true},
		{"db-query", true},
		{"db.query", true},
		{"a!#$%&'*+-.^_`|~b", true},
		{"123", true},
		{"a b", false},
		{"a,b", false},
		{"a;b", false},
		{`a"b`, false},
		{"a=b", false},
		{"a\tb", false},
	}
	for _, tt := range tests {
		if got := isToken(tt.s); got != tt.want {
			t.Errorf("isToken(%q) = %v, want %v", tt.s, got, tt.want)
		}
	}
}
