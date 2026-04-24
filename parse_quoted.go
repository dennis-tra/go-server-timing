package servertiming

import (
	"errors"
	"strings"
)

// scanQuoted reads a quoted-string starting at index i, where s[i]
// must be the opening DQUOTE. It returns the unescaped content, the
// index immediately after the closing DQUOTE, and an error if the
// input is not a well-formed quoted-string.
//
//	quoted-string = DQUOTE *( qdtext / quoted-pair ) DQUOTE
//	qdtext        = HTAB / SP / %x21 / %x23-5B / %x5D-7E / obs-text
//	quoted-pair   = "\" ( HTAB / SP / VCHAR / obs-text )
//
// The scanner is lenient about qdtext content (any byte other than
// DQUOTE and backslash is accepted) and strict about termination:
// unterminated strings and trailing backslashes return an error.
func scanQuoted(s string, i int) (value string, next int, err error) {
	if i >= len(s) || s[i] != '"' {
		return "", i, errors.New("servertiming: expected DQUOTE")
	}
	i++
	var b strings.Builder
	for i < len(s) {
		c := s[i]
		if c == '"' {
			return b.String(), i + 1, nil
		}
		if c == '\\' {
			if i+1 >= len(s) {
				return "", i, errors.New("servertiming: unterminated quoted-pair")
			}
			b.WriteByte(s[i+1])
			i += 2
			continue
		}
		b.WriteByte(c)
		i++
	}
	return "", i, errors.New("servertiming: unterminated quoted-string")
}

// splitList splits s on top-level commas, respecting quoted strings.
// Surrounding optional whitespace is trimmed from each piece and
// empty pieces (from consecutive commas) are dropped, matching the
// RFC 7230 section 7 list-rule semantics.
func splitList(s string) []string {
	var out []string
	start := 0
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inQuote {
			if c == '\\' && i+1 < len(s) {
				i++
				continue
			}
			if c == '"' {
				inQuote = false
			}
			continue
		}
		if c == '"' {
			inQuote = true
			continue
		}
		if c == ',' {
			if piece := strings.TrimSpace(s[start:i]); piece != "" {
				out = append(out, piece)
			}
			start = i + 1
		}
	}
	if piece := strings.TrimSpace(s[start:]); piece != "" {
		out = append(out, piece)
	}
	return out
}

// splitMetric splits a single metric entry on top-level semicolons,
// respecting quoted strings. The first element is the metric-name
// (with surrounding OWS not yet trimmed); subsequent elements are
// params. Empty elements may appear and are handled by the caller.
func splitMetric(s string) []string {
	var out []string
	start := 0
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inQuote {
			if c == '\\' && i+1 < len(s) {
				i++
				continue
			}
			if c == '"' {
				inQuote = false
			}
			continue
		}
		if c == '"' {
			inQuote = true
			continue
		}
		if c == ';' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

// findTopLevelEquals returns the index of the first '=' in s that is
// not inside a quoted string, or -1 if none exists.
func findTopLevelEquals(s string) int {
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inQuote {
			if c == '\\' && i+1 < len(s) {
				i++
				continue
			}
			if c == '"' {
				inQuote = false
			}
			continue
		}
		if c == '"' {
			inQuote = true
			continue
		}
		if c == '=' {
			return i
		}
	}
	return -1
}
