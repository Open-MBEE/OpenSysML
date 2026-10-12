package semantics

import (
	"math"
	"math/big"
	"testing"
)

// Every Value a Packed can hold unpacks to the Value it was packed from, field
// for field, the big form sharing its immutable big.Rat.
func TestPackedRoundTrip(t *testing.T) {
	bigRat, _ := new(big.Rat).SetString("12345678901234567890123/7")
	values := []Value{
		{},
		{Kind: ValInfinity},
		IntValue(0), IntValue(1), IntValue(-1), IntValue(math.MaxInt64), IntValue(math.MinInt64),
		bigVal("1180591620717411303424"), bigVal("-1180591620717411303424"),
		mustFrac(t, 1, 3), mustFrac(t, -7, 2), mustFrac(t, 6, 3), mustFrac(t, math.MaxInt64, math.MaxUint32),
		RatValue(bigRat),
		{Kind: ValReal, Real: 0}, {Kind: ValReal, Real: math.Copysign(0, -1)}, {Kind: ValReal, Real: 1.5},
		{Kind: ValReal, Real: math.Inf(1)}, {Kind: ValReal, Real: math.Inf(-1)}, {Kind: ValReal, Real: math.NaN()},
		{Kind: ValBool, Bool: true}, {Kind: ValBool, Bool: false},
	}
	for _, v := range values {
		got := v.Pack().Unpack()
		if v.Kind == ValReal && math.IsNaN(v.Real) {
			if got.Kind != ValReal || !math.IsNaN(got.Real) || math.Float64bits(got.Real) != math.Float64bits(v.Real) {
				t.Errorf("NaN round-trips to %+v", got)
			}
			continue
		}
		if got != v {
			t.Errorf("%+v round-trips to %+v", v, got)
		}
		if got.ext != v.ext {
			t.Errorf("%+v round-trips to another big.Rat", v)
		}
	}
}

func mustFrac(t *testing.T, num, den int64) Value {
	t.Helper()
	v, ok := FracValue(num, den)
	if !ok {
		t.Fatalf("FracValue(%d, %d) declined", num, den)
	}
	return v
}
