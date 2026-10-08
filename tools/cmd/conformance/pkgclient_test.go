package main

import (
	"math"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
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

func TestRenderSpanToProtoChecksInt32Bounds(t *testing.T) {
	span := &opensysml.Span{
		File: "model.sysml", StartLine: math.MaxInt32, StartCol: 2, EndLine: 3, EndCol: 4,
	}
	got, err := renderSpanToProto(span)
	if err != nil {
		t.Fatalf("renderSpanToProto() error = %v", err)
	}
	if got.StartLine != math.MaxInt32 {
		t.Fatalf("StartLine = %d, want %d", got.StartLine, math.MaxInt32)
	}

	if math.MaxInt > math.MaxInt32 {
		span.StartLine = int(math.MaxInt32) + 1
		if _, err := renderSpanToProto(span); err == nil {
			t.Fatal("renderSpanToProto() accepted a coordinate outside int32 range")
		}
	}
}
