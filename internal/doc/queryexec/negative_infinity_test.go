package queryexec

import (
	"math"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// TestConstantValueNegativeInfinity: `-*` is a Real cell printing as written,
// while `*` stays the unbounded value.
func TestConstantValueNegativeInfinity(t *testing.T) {
	negative, ok := constantValue(semantics.NegativeInfinity())
	if !ok || negative.Kind() != ValueReal || !math.IsInf(negative.real, -1) {
		t.Fatalf("constantValue(-*) = %+v, %v; want the real -Inf", negative, ok)
	}
	if text, ok := negative.Text(nil); !ok || text != "-*" {
		t.Errorf("text of -* = %q, %v", text, ok)
	}
	positive, ok := constantValue(semantics.PositiveInfinity())
	if !ok || positive.Kind() != ValueInfinity {
		t.Fatalf("constantValue(*) = %+v, %v; want the unbounded value", positive, ok)
	}
	if text, ok := positive.Text(nil); !ok || text != "*" {
		t.Errorf("text of * = %q, %v", text, ok)
	}
}
