package opensysml_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

// A nil argument to a number constructor is zero, as the zero values read.
func TestNumberConstructorsTreatNilAsZero(t *testing.T) {
	if got := opensysml.NewInteger(nil); got != opensysml.Int(0) {
		t.Errorf("NewInteger(nil) = %#v, want Int(0)", got)
	}
	q := opensysml.NewRational(nil)
	if q.Rat().Sign() != 0 || q.Float64() != 0 {
		t.Errorf("NewRational(nil) = %v, want zero", q.Rat())
	}
	var zero opensysml.Rational
	if q.Rat().Cmp(zero.Rat()) != 0 {
		t.Errorf("NewRational(nil) = %v, want the zero Rational %v", q.Rat(), zero.Rat())
	}
	var big opensysml.BigInt
	if big.Int().Sign() != 0 || big.String() != "0" {
		t.Errorf("zero BigInt = %s, want 0", big.String())
	}
}
