package main

import (
	"math"
	"testing"
)

func TestTraceDroppedCountToInt32(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	tests := []struct {
		name    string
		dropped int
		want    int32
	}{
		{name: "negative", dropped: -1, want: 0},
		{name: "in range", dropped: 12, want: 12},
		{name: "maximum int", dropped: maxInt, want: math.MaxInt32},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := traceDroppedCountToInt32(test.dropped); got != test.want {
				t.Fatalf("traceDroppedCountToInt32(%d) = %d, want %d", test.dropped, got, test.want)
			}
		})
	}
}
