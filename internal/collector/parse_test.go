package collector

import "testing"

func TestParseBytes(t *testing.T) {
	cases := map[string]float64{
		"1024":     1024,
		"10M":      10 << 20,
		"1.5GiB":   1.5 * (1 << 30),
		"500k":     500 << 10,
		"12.3 KiB": 12.3 * (1 << 10),
		"7B":       7,
	}
	for in, want := range cases {
		got, err := ParseBytes(in)
		if err != nil || got != want {
			t.Errorf("ParseBytes(%q) = %v, %v; want %v", in, got, err, want)
		}
	}

	for _, in := range []string{"", "abc", "10X"} {
		if _, err := ParseBytes(in); err == nil {
			t.Errorf("ParseBytes(%q) expected error", in)
		}
	}
}

func TestParseDuration(t *testing.T) {
	cases := map[string]float64{
		"1w2d3h4m5s": 7*86400 + 2*86400 + 3*3600 + 4*60 + 5,
		"5m10s300ms": 310.3,
		"2d03:04:05": 2*86400 + 3*3600 + 4*60 + 5,
		"00:00:10":   10,
		"1w00:00:01": 7*86400 + 1,
		"45s":        45,
		"1d":         86400,
		"10ms":       0.01,
		"00:01:02.5": 62.5,
		" 3h ":       3 * 3600,
		"1w2d":       9 * 86400,
		"3m":         180,
		"999us":      999e-6,
	}
	for in, want := range cases {
		got, err := ParseDuration(in)
		if err != nil || got < want-1e-9 || got > want+1e-9 {
			t.Errorf("ParseDuration(%q) = %v, %v; want %v", in, got, err, want)
		}
	}

	for _, in := range []string{"", "abc", "10x", "1:2", "h"} {
		if _, err := ParseDuration(in); err == nil {
			t.Errorf("ParseDuration(%q) expected error", in)
		}
	}
}

func TestParsePercent(t *testing.T) {
	for in, want := range map[string]float64{"15": 15, "15%": 15, " 3% ": 3} {
		got, err := ParsePercent(in)
		if err != nil || got != want {
			t.Errorf("ParsePercent(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
}
