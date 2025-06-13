package collector

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// ParseBytes converts a MikroTik byte annotation string to uint64 bytes.
// Supported suffixes: (case-insensitive) k, m, g, t
// Example inputs: "10M", "1.5G", "500k", "1024"
func ParseBytes(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty string")
	}

	// Find the index where the digits end
	var i int
	for i = 0; i < len(s); i++ {
		if !unicode.IsDigit(rune(s[i])) && s[i] != '.' {
			break
		}
	}

	// Parse the numeric part
	numStr := s[:i]
	if numStr == "" {
		return 0, errors.New("no numeric value found")
	}

	value, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, err
	}

	// Parse the suffix
	suffix := strings.ToLower(strings.TrimSpace(s[i:]))
	var multiplier float64

	switch suffix {
	case "", "b":
		multiplier = 1
	case "k", "kb":
		multiplier = math.Pow(2, 10) // 1024
	case "m", "mb":
		multiplier = math.Pow(2, 20) // 1048576
	case "g", "gb":
		multiplier = math.Pow(2, 30) // 1073741824
	case "t", "tb":
		multiplier = math.Pow(2, 40) // 1099511627776
	default:
		return 0, errors.New("invalid suffix: " + suffix)
	}

	// Calculate the total bytes
	total := value * multiplier

	return total, nil
}
