package servertiming

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseHeader parses a single Server-Timing header value into a new
// Header. Multiple metrics separated by commas are all recorded.
//
// Parsing is tolerant: malformed metrics are skipped, with errors
// collected via errors.Join and returned as a non-fatal error. The
// returned *Header always contains whatever metrics could be salvaged,
// and is non-nil even when the error is non-nil.
//
// Duplicate params on a single metric follow last-wins semantics. A
// "dur" value that does not parse as a float is preserved as
// Extra["dur"] and a soft error is returned.
func ParseHeader(value string) (*Header, error) {
	h := NewHeader()
	errs := parseInto(h, value)
	return h, errors.Join(errs...)
}

// ParseHeaders parses a multi-valued Server-Timing header (e.g. the
// result of http.Header.Values). Metrics from all values are merged
// into the returned Header in input order.
func ParseHeaders(values []string) (*Header, error) {
	h := NewHeader()
	var errs []error
	for _, v := range values {
		errs = append(errs, parseInto(h, v)...)
	}
	return h, errors.Join(errs...)
}

func parseInto(h *Header, value string) []error {
	var errs []error
	for _, raw := range splitList(value) {
		m, err := parseMetric(raw)
		if err != nil {
			errs = append(errs, err)
		}
		if m != nil {
			h.Add(m)
		}
	}
	return errs
}

// parseMetric parses one server-timing-metric entry. Returns a
// non-nil Metric when the metric-name is valid, even if some params
// failed to parse; per-param errors are joined into the returned
// error.
func parseMetric(s string) (*Metric, error) {
	parts := splitMetric(s)
	if len(parts) == 0 {
		return nil, fmt.Errorf("servertiming: empty metric")
	}
	name := strings.TrimSpace(parts[0])
	if !isToken(name) {
		return nil, fmt.Errorf("servertiming: invalid metric name %q", name)
	}
	m := NewMetric(name)
	var errs []error
	for _, p := range parts[1:] {
		if err := applyParam(m, p); err != nil {
			errs = append(errs, fmt.Errorf("metric %q: %w", name, err))
		}
	}
	return m, errors.Join(errs...)
}

// applyParam parses a single "name[=value]" fragment and applies it
// to m. The name is case-insensitive. A missing value is stored as
// an empty Extra entry. Values may be a token or a quoted-string.
func applyParam(m *Metric, param string) error {
	param = strings.TrimSpace(param)
	if param == "" {
		return nil
	}

	eqIdx := findTopLevelEquals(param)
	var (
		name, rawValue string
		hasValue       bool
	)
	if eqIdx == -1 {
		name = strings.TrimSpace(param)
	} else {
		name = strings.TrimSpace(param[:eqIdx])
		rawValue = strings.TrimLeft(param[eqIdx+1:], " \t")
		hasValue = true
	}

	if !isToken(name) {
		return fmt.Errorf("invalid param name %q", name)
	}

	value, err := parseParamValue(rawValue, hasValue)
	if err != nil {
		return fmt.Errorf("param %q: %w", name, err)
	}

	switch strings.ToLower(name) {
	case "dur":
		if !hasValue {
			return fmt.Errorf("param %q: missing value", name)
		}
		ms, err := strconv.ParseFloat(value, 64)
		if err != nil {
			m.Duration = 0
			m.putExtra("dur", value)
			return fmt.Errorf("param %q: invalid numeric value %q", name, value)
		}
		m.Duration = time.Duration(ms * float64(time.Millisecond))
		delete(m.Extra, "dur")
	case "desc":
		m.Description = value
		delete(m.Extra, "desc")
	default:
		m.putExtra(strings.ToLower(name), value)
	}
	return nil
}

// parseParamValue resolves a trimmed raw value into its unquoted form.
// An empty raw value is only legal when hasValue is false (the param
// had no '='); `name=` with a truly empty RHS is a malformed token.
func parseParamValue(rawValue string, hasValue bool) (string, error) {
	if !hasValue {
		return "", nil
	}
	if rawValue == "" {
		return "", errors.New("empty value after '='")
	}
	if rawValue[0] == '"' {
		v, next, err := scanQuoted(rawValue, 0)
		if err != nil {
			return "", err
		}
		if trailing := strings.TrimSpace(rawValue[next:]); trailing != "" {
			return "", fmt.Errorf("unexpected content after quoted value: %q", trailing)
		}
		return v, nil
	}
	if !isToken(rawValue) {
		return "", fmt.Errorf("invalid token value %q", rawValue)
	}
	return rawValue, nil
}
