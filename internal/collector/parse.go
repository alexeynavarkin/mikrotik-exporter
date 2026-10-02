package collector

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// ParseBytes converts a MikroTik byte annotation string to bytes.
// Supported suffixes (case-insensitive): b, k, kb, kib, m, mb, mib, g, gb, gib, t, tb, tib.
// All of them are treated as powers of 1024, as RouterOS does.
// Example inputs: "10M", "1.5GiB", "500k", "1024".
func ParseBytes(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty string")
	}

	i := numericPrefixLen(s)
	if i == 0 {
		return 0, fmt.Errorf("no numeric value found in %q", s)
	}

	value, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return 0, err
	}

	var multiplier float64
	switch suffix := strings.ToLower(strings.TrimSpace(s[i:])); suffix {
	case "", "b":
		multiplier = 1
	case "k", "kb", "kib":
		multiplier = 1 << 10
	case "m", "mb", "mib":
		multiplier = 1 << 20
	case "g", "gb", "gib":
		multiplier = 1 << 30
	case "t", "tb", "tib":
		multiplier = 1 << 40
	default:
		return 0, fmt.Errorf("invalid suffix %q", suffix)
	}

	return value * multiplier, nil
}

// ParseDuration converts a RouterOS duration to seconds.
// Example inputs: "1w2d3h4m5s", "5m10s300ms", "2d03:04:05", "00:00:10".
func ParseDuration(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty string")
	}

	var total float64

	// Optional trailing clock part: HH:MM:SS[.fraction].
	if colon := strings.IndexByte(s, ':'); colon >= 0 {
		start := colon
		for start > 0 && isDigit(s[start-1]) {
			start--
		}

		parts := strings.Split(s[start:], ":")
		if len(parts) != 3 {
			return 0, fmt.Errorf("invalid clock format in %q", s)
		}

		multipliers := []float64{3600, 60, 1}
		for i, part := range parts {
			value, err := strconv.ParseFloat(part, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid clock format in %q: %w", s, err)
			}
			total += value * multipliers[i]
		}

		s = s[:start]
	}

	for len(s) > 0 {
		i := numericPrefixLen(s)
		if i == 0 {
			return 0, fmt.Errorf("expected number in %q", s)
		}

		value, err := strconv.ParseFloat(s[:i], 64)
		if err != nil {
			return 0, err
		}
		s = s[i:]

		j := 0
		for j < len(s) && isLetter(s[j]) {
			j++
		}

		var multiplier float64
		switch unit := s[:j]; unit {
		case "w":
			multiplier = 7 * 24 * 3600
		case "d":
			multiplier = 24 * 3600
		case "h":
			multiplier = 3600
		case "m":
			multiplier = 60
		case "s":
			multiplier = 1
		case "ms":
			multiplier = 1e-3
		case "us":
			multiplier = 1e-6
		case "ns":
			multiplier = 1e-9
		default:
			return 0, fmt.Errorf("invalid duration unit %q", unit)
		}
		s = s[j:]

		total += value * multiplier
	}

	return total, nil
}

// ParsePercent parses values like "15" or "15%".
func ParsePercent(s string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(s), "%"), 64)
}

func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(s), 64)
}

func parseBool(s string) bool {
	return s == "true" || s == "yes"
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// emit parses raw with parse and sends the metric, silently skipping
// missing or unparsable values.
func emit(
	ch chan<- prometheus.Metric,
	desc *prometheus.Desc,
	valueType prometheus.ValueType,
	raw string,
	parse func(string) (float64, error),
	labels ...string,
) {
	if raw == "" {
		return
	}

	value, err := parse(raw)
	if err != nil {
		return
	}

	ch <- prometheus.MustNewConstMetric(desc, valueType, value, labels...)
}

func numericPrefixLen(s string) int {
	i := 0
	for i < len(s) && (isDigit(s[i]) || s[i] == '.') {
		i++
	}
	return i
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
