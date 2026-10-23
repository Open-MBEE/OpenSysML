package solve

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

// crateSource derives one attribute from three others nothing gives a value
// to, under bounds on the three and a condition on the derived one.
const crateSource = `package Box {
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;
	part def Crate {
		attribute height : LengthValue;
		attribute width : LengthValue;
		attribute length : LengthValue;
		attribute volume : VolumeValue = height * width * length;
		assert constraint bounds {
			height >= 0.1 [m] and height <= 1.0 [m] and
			width >= 1.0 [m] and width <= 1.1 [m] and
			length >= 1.0 [m] and length <= 1.1 [m]
		}
		assert constraint fits { volume >= 1.0 [m**3] and volume <= 2.0 [m**3] }
	}
	part crateUnset : Crate;
	part crateBad : Crate {
		attribute :>> height = 0.1 [m];
		attribute :>> width = 1.0 [m];
		attribute :>> length = 1.0 [m];
	}
}`

// TestFixedLeavesAnUndeterminedDerivedValueUnfixed: a declared value that does
// not evaluate because a feature it reads holds nothing is no pin — there is
// no literal to fix it to — but an Unfixed naming that feature, as a chain
// reading it reports, which UnfixedRead refuses where the read is unconditional.
func TestFixedLeavesAnUndeterminedDerivedValueUnfixed(t *testing.T) {
	ctx, idx := fixture(t, "crate.sysml", crateSource)
	crate := symbolNamed(t, idx, "Box::Crate")
	pins, unfixed := Fixed(ctx, crate, nil)
	for _, p := range pins {
		if p.Name == "volume" {
			t.Fatalf("volume was fixed to %v, want it unfixed: nothing gives height a value", p.Value)
		}
	}
	var volume Unfixed
	for _, u := range unfixed {
		switch u.Name {
		case "volume":
			volume = u
		case "isSolid":
			// Inherited from Items::Item, whose voids nothing gives a value to.
		default:
			t.Fatalf("unfixed = %+v, want volume alone among the crate's own", unfixed)
		}
	}
	if volume.Feature == nil {
		t.Fatalf("unfixed = %+v, want volume among them", unfixed)
	}
	var noValue *runtime.NoValueError
	if !errors.As(volume.Err, &noValue) || noValue.Symbol != symbolNamed(t, idx, "Box::Crate::height") {
		t.Fatalf("volume.Err = %v, want the read of height to be what fails", volume.Err)
	}
	if !errors.Is(volume.Err, runtime.ErrNoValue) || volume.Reason != volume.Err.Error() || !strings.Contains(volume.Reason, "no value for feature height") {
		t.Fatalf("Reason = %q, Err = %v", volume.Reason, volume.Err)
	}

	fits := symbolNamed(t, idx, "Box::Crate::fits")
	q, err := ConstraintWith(ctx, fits, fits.OwnerScope, pins)
	if err != nil {
		t.Fatalf("translate fits: %v", err)
	}
	if err := UnfixedRead(ctx, q, unfixed); err == nil || !errors.Is(err, volume.Err) || !strings.Contains(err.Error(), "volume") {
		t.Fatalf("UnfixedRead fits = %v, want a refusal naming volume", err)
	}
	bounds := symbolNamed(t, idx, "Box::Crate::bounds")
	unrelated, err := ConstraintWith(ctx, bounds, bounds.OwnerScope, pins)
	if err != nil {
		t.Fatalf("translate bounds: %v", err)
	}
	if err := UnfixedRead(ctx, unrelated, unfixed); err != nil {
		t.Fatalf("UnfixedRead bounds = %v, want nil: it does not read volume", err)
	}
}
