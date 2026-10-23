package repl

import (
	"strings"
	"testing"
)

// crateModel derives volume from three dimensions nothing gives a value to,
// under bounds on the three and a condition on the derived one.
const crateModel = `
package Box {
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
}
`

// TestConfigureReadsADerivedValueWhoseOperandsAreUnset: a declared value that
// does not evaluate because the features it reads hold nothing is no value to
// fix, so the commands reading the object's fixed values answer rather than
// refusing to fix it.
func TestConfigureReadsADerivedValueWhoseOperandsAreUnset(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, crateModel)
	run(t, s, "%instantiate Box::crateUnset")
	for _, cmd := range []string{"%configure Box::crateUnset", "%check Box::crateUnset", "%explain Box::crateUnset"} {
		out := run(t, s, cmd)
		if strings.Contains(out, "has no literal in the term language") || strings.Contains(out, "cannot be fixed") {
			t.Errorf("%s refused to fix the derived value:\n%s", cmd, out)
		}
	}
	wants(t, run(t, s, "%configure Box::crateUnset"), "no variation point is read")
	wants(t, run(t, s, "%check Box::crateUnset"), "✓ Part crateUnset is satisfiable", "Box::Crate::height =")
}
