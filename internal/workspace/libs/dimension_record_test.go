package libs

import (
	"bytes"
	"encoding/gob"
	"testing"
)

// TestDimensionFactsRoundTrip locks the distinction the restored check depends
// on: a recorded dimension with no factors is a determined dimensionless one,
// and must not come back as the nil that means "nothing was recorded".
func TestDimensionFactsRoundTrip(t *testing.T) {
	rec := IndexRecord{Facts: []factRecord{
		{FQN: "Dimensionless", Dimension: &dimensionFacts{}},
		{FQN: "Length", Dimension: &dimensionFacts{
			Factors: []dimensionFactor{{FQN: "ISQBase::length", Exponent: 1}},
		}},
		{FQN: "Undetermined"},
	}}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(rec); err != nil {
		t.Fatalf("encode: %v", err)
	}
	var out IndexRecord
	if err := gob.NewDecoder(&buf).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := out.Facts[0].Dimension; got == nil || len(got.Factors) != 0 {
		t.Errorf("a dimensionless dimension restored as %v, want an empty one", got)
	}
	if got := out.Facts[1].Dimension; got == nil || len(got.Factors) != 1 ||
		got.Factors[0].FQN != "ISQBase::length" || got.Factors[0].Exponent != 1 {
		t.Errorf("factors restored as %v", got)
	}
	if got := out.Facts[2].Dimension; got != nil {
		t.Errorf("an unrecorded dimension restored as %v, want nil", got)
	}
}
