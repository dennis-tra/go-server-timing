package servertiming

import (
	"slices"
	"strconv"
	"strings"
	"time"
)

// Metric is a single server-timing-metric entry.
//
// Name must be a valid HTTP token (RFC 7230 section 3.2.6); it is
// emitted verbatim. Duration is serialized as the "dur" parameter in
// fractional milliseconds and omitted when zero. Description is
// serialized as "desc", quoted when it contains non-token characters.
// Extra carries any additional params preserved by the parser, with
// keys normalized to lowercase.
type Metric struct {
	Name        string
	Duration    time.Duration
	Description string
	Extra       map[string]string

	startTime time.Time
}

// NewMetric returns a Metric with only the Name set.
func NewMetric(name string) *Metric {
	return &Metric{Name: name}
}

// WithDuration sets Duration and returns the receiver. Any raw value
// previously stored under Extra["dur"] is cleared.
func (m *Metric) WithDuration(d time.Duration) *Metric {
	m.Duration = d
	delete(m.Extra, "dur")
	return m
}

// WithDesc sets Description and returns the receiver. Any raw value
// previously stored under Extra["desc"] is cleared.
func (m *Metric) WithDesc(desc string) *Metric {
	m.Description = desc
	delete(m.Extra, "desc")
	return m
}

// WithParam sets an arbitrary param. The name is lowercased so writes
// are case-insensitive. Passing "dur" or "desc" routes to Duration or
// Description; a "dur" value that does not parse as a float is stored
// under Extra["dur"] instead and Duration is cleared.
func (m *Metric) WithParam(name, value string) *Metric {
	switch strings.ToLower(name) {
	case "dur":
		if ms, err := strconv.ParseFloat(value, 64); err == nil {
			m.Duration = time.Duration(ms * float64(time.Millisecond))
			delete(m.Extra, "dur")
			return m
		}
		m.Duration = 0
		m.putExtra("dur", value)
	case "desc":
		m.Description = value
		delete(m.Extra, "desc")
	default:
		m.putExtra(strings.ToLower(name), value)
	}
	return m
}

func (m *Metric) putExtra(key, value string) {
	if m.Extra == nil {
		m.Extra = make(map[string]string)
	}
	m.Extra[key] = value
}

// Start stamps the current time. A subsequent Stop sets Duration to
// the elapsed time. Typical usage:
//
//	defer servertiming.FromContext(ctx).NewMetric("db").Start().Stop()
func (m *Metric) Start() *Metric {
	m.startTime = time.Now()
	return m
}

// Stop sets Duration to time.Since the last Start call. If Start was
// not called Duration is left unchanged.
func (m *Metric) Stop() *Metric {
	if !m.startTime.IsZero() {
		m.Duration = time.Since(m.startTime)
	}
	return m
}

// String returns m serialized as a single server-timing-metric entry.
// Values that are not valid tokens are emitted as quoted strings.
// Duration is rendered in fractional milliseconds using the shortest
// representation that round-trips through strconv.ParseFloat. Extra
// params are emitted in alphabetical order for deterministic output;
// callers must not rely on this ordering as it is not guaranteed by
// the public contract.
func (m *Metric) String() string {
	var b strings.Builder
	writeName(&b, m.Name)
	if m.Duration != 0 {
		b.WriteString(";dur=")
		b.WriteString(formatDuration(m.Duration))
	}
	if m.Description != "" {
		b.WriteString(";desc=")
		writeValue(&b, m.Description)
	}
	if len(m.Extra) > 0 {
		keys := make([]string, 0, len(m.Extra))
		for k := range m.Extra {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			b.WriteByte(';')
			b.WriteString(k)
			b.WriteByte('=')
			writeValue(&b, m.Extra[k])
		}
	}
	return b.String()
}

func formatDuration(d time.Duration) string {
	return strconv.FormatFloat(float64(d)/float64(time.Millisecond), 'f', -1, 64)
}

func writeName(b *strings.Builder, name string) {
	if isToken(name) {
		b.WriteString(name)
		return
	}
	writeQuoted(b, name)
}

func writeValue(b *strings.Builder, value string) {
	if value != "" && isToken(value) {
		b.WriteString(value)
		return
	}
	writeQuoted(b, value)
}

// writeQuoted emits DQUOTE *(qdtext / quoted-pair) DQUOTE, escaping
// internal DQUOTE and backslash with a leading backslash.
func writeQuoted(b *strings.Builder, value string) {
	b.WriteByte('"')
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c == '"' || c == '\\' {
			b.WriteByte('\\')
		}
		b.WriteByte(c)
	}
	b.WriteByte('"')
}

// isToken reports whether s is a non-empty RFC 7230 token.
//
//	token  = 1*tchar
//	tchar  = "!" / "#" / "$" / "%" / "&" / "'" / "*"
//	      / "+" / "-" / "." / "^" / "_" / "`" / "|" / "~"
//	      / DIGIT / ALPHA
func isToken(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isTChar(s[i]) {
			return false
		}
	}
	return true
}

func isTChar(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z',
		c >= 'A' && c <= 'Z',
		c >= '0' && c <= '9':
		return true
	}
	switch c {
	case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.',
		'^', '_', '`', '|', '~':
		return true
	}
	return false
}
