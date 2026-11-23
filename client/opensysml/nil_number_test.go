package opensysml_test

import (
	"math/big"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

// A nil big number is zero rather than a panic in the constructor.
func TestNumberConstructorsReadNilAsZero(t *testing.T) {
	if got := opensysml.NewInteger(nil); got != opensysml.Int(0) {
		t.Errorf("NewInteger(nil) = %#v, want Int(0)", got)
	}
	rational := opensysml.NewRational(nil)
	if rat := rational.Rat(); rat == nil || rat.Sign() != 0 {
		t.Errorf("NewRational(nil).Rat() = %v, want 0", rat)
	}
	if text := rational.String(); text != opensysml.NewRational(new(big.Rat)).String() {
		t.Errorf("NewRational(nil) prints %q", text)
	}
}
