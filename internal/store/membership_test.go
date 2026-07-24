package store

import (
	"math"
	"testing"
)

func TestFeeAmountCents(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		amount int64
		bps    int
		want   int64
	}{
		{name: "zero amount", amount: 0, bps: 300, want: 0},
		{name: "zero fee", amount: 10_000, bps: 0, want: 0},
		{name: "ordinary withdrawal three percent", amount: 10_000, bps: 300, want: 300},
		{name: "ordinary service five percent", amount: 10_000, bps: 500, want: 500},
		{name: "rounds fractional cent upward", amount: 1, bps: 300, want: 1},
		{name: "one hundred percent", amount: 10_000, bps: 10_000, want: 10_000},
		{name: "fee above maximum is capped", amount: 10_000, bps: 20_000, want: 10_000},
		{name: "large amount does not overflow", amount: math.MaxInt64, bps: 300, want: 276_701_161_105_643_275},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := feeAmountCents(test.amount, test.bps); got != test.want {
				t.Fatalf("feeAmountCents(%d, %d) = %d; want %d", test.amount, test.bps, got, test.want)
			}
		})
	}
}
