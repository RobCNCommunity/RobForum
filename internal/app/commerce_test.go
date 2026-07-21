package app

import "testing"

func TestParseMoneyCents(t *testing.T) {
	tests := map[string]int64{"0.01": 1, "1": 100, "9.9": 990, "12.34": 1234}
	for raw, want := range tests {
		got, err := parseMoneyCents(raw)
		if err != nil || got != want {
			t.Fatalf("parseMoneyCents(%q) = %d, %v; want %d", raw, got, err, want)
		}
	}
	for _, raw := range []string{"", "-1", "1.234", "x"} {
		if _, err := parseMoneyCents(raw); err == nil {
			t.Fatalf("parseMoneyCents(%q) accepted invalid money", raw)
		}
	}
}
