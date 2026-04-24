package servertiming

import (
	"testing"
)

func TestScanQuoted(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		next    int
		wantErr bool
	}{
		{"empty string", `""`, "", 2, false},
		{"simple", `"abc"`, "abc", 5, false},
		{"with space", `"a b"`, "a b", 5, false},
		{"escaped quote", `"a\"b"`, `a"b`, 6, false},
		{"escaped backslash", `"a\\b"`, `a\b`, 6, false},
		{"escaped inside", `"\\n"`, `\n`, 5, false},
		{"commas and semicolons preserved", `"a,b;c=d"`, "a,b;c=d", 9, false},
		{"trailing content ignored", `"abc"xyz`, "abc", 5, false},
		{"unterminated string", `"abc`, "", 4, true},
		{"unterminated escape", `"a\`, "", 3, true},
		{"not a quote", `abc`, "", 0, true},
		{"empty input", ``, "", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, next, err := scanQuoted(tt.in, 0)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got != tt.want {
				t.Errorf("value = %q, want %q", got, tt.want)
			}
			if next != tt.next {
				t.Errorf("next = %d, want %d", next, tt.next)
			}
		})
	}
}

func TestSplitList(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a,b", []string{"a", "b"}},
		{"a, b", []string{"a", "b"}},
		{" a , b ", []string{"a", "b"}},
		{",,a,", []string{"a"}},
		{",", nil},
		{`"a,b"`, []string{`"a,b"`}},
		{`a;desc="x,y",b`, []string{`a;desc="x,y"`, "b"}},
		{`a,"b,c"`, []string{"a", `"b,c"`}},
		{`a;desc="has\"quote,inside",b;dur=5`, []string{`a;desc="has\"quote,inside"`, "b;dur=5"}},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := splitList(tt.in)
			if !equalStringSlices(got, tt.want) {
				t.Errorf("splitList(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSplitMetric(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"db", []string{"db"}},
		{"db;dur=5", []string{"db", "dur=5"}},
		{`db;desc="a;b";dur=5`, []string{"db", `desc="a;b"`, "dur=5"}},
		{"db;;dur=5", []string{"db", "", "dur=5"}},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := splitMetric(tt.in)
			if !equalStringSlices(got, tt.want) {
				t.Errorf("splitMetric(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFindTopLevelEquals(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"", -1},
		{"foo", -1},
		{"foo=bar", 3},
		{`"foo=bar"`, -1},
		{`foo="a=b"`, 3},
		{`"a=b"=c`, 5},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := findTopLevelEquals(tt.in); got != tt.want {
				t.Errorf("findTopLevelEquals(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
