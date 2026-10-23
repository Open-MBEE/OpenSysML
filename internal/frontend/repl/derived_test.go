package repl

import (
	"math/big"
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
// refusing to fix it; the value is defined by its equation, so the witness's
// volume is the product of the dimensions chosen within their bounds.
func TestConfigureReadsADerivedValueWhoseOperandsAreUnset(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, crateModel)
	run(t, s, "%instantiate Box::crateUnset")
	for _, cmd := range []string{"%configure Box::crateUnset", "%check Box::crateUnset", "%explain Box::crateUnset", "%solve Box::Crate::fits"} {
		out := run(t, s, cmd)
		if strings.Contains(out, "has no literal in the term language") || strings.Contains(out, "cannot be fixed") {
			t.Errorf("%s refused to fix the derived value:\n%s", cmd, out)
		}
	}
	wants(t, run(t, s, "%configure Box::crateUnset"), "no variation point is read")
	check := run(t, s, "%check Box::crateUnset")
	wants(t, check, "✓ Part crateUnset is satisfiable", "Defined by the model:", "Box::Crate::volume = height * width * length")
	values := assignedValues(t, check, "Box::Crate::height", "Box::Crate::width", "Box::Crate::length", "Box::Crate::volume")
	product := new(big.Rat).Mul(values["Box::Crate::height"], values["Box::Crate::width"])
	product.Mul(product, values["Box::Crate::length"])
	if values["Box::Crate::volume"].Cmp(product) != 0 {
		t.Errorf("volume = %s, want the product %s of the dimensions:\n%s", values["Box::Crate::volume"], product, check)
	}
	if h := values["Box::Crate::height"]; h.Cmp(big.NewRat(1, 10)) < 0 || h.Cmp(big.NewRat(1, 1)) > 0 {
		t.Errorf("height = %s, want one within bounds:\n%s", h, check)
	}
	solve := run(t, s, "%solve Box::Crate::fits")
	wants(t, solve, "✓ Constraint fits has values satisfying it", "Defined by the model:", "Box::Crate::volume = height * width * length")
	if strings.Contains(solve, "Left free") {
		t.Errorf("%%solve left the derived value free:\n%s", solve)
	}
}

// TestSolveNamesTheHeldValuesInAConflictWithTheEquation: over an object holding
// dimensions whose product no volume in range matches, the conflict names the
// held values and the defining equation, with the declaration's position.
func TestSolveNamesTheHeldValuesInAConflictWithTheEquation(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, crateModel)
	run(t, s, "%instantiate Box::crateBad")
	solve := run(t, s, "%solve Box::Crate::fits")
	wants(t, solve, "✗ Constraint fits has no values consistent with the values already fixed",
		"Box::Crate::height = 0.1 [m]  (held by object 1)", "Defined by the model:",
		"Box::Crate::volume = height * width * length", "In the conflict: Box::Crate::height = 0.1 [m], Box::Crate::length = 1.0 [m], Box::Crate::width = 1.0 [m]",
		"Against: defining equation Box::Crate::volume == height * width * length, declared by the model of volume at")
	if strings.Contains(solve, "Synthesised:") {
		t.Errorf("an unsatisfiable query synthesised values:\n%s", solve)
	}
	explain := run(t, s, "%explain Box::crateBad")
	wants(t, explain, "✗ Part crateBad is unsatisfiable", "fixed value: `Box::crateBad::height == 0.1 [m], declared by the model`",
		"defining equation: `Box::Crate::volume == height * width * length, declared by the model` — feature Box::Crate::volume, at ", ":10:36")
}

// assignedValues reads the exact values the report assigns to names, each
// written `name = magnitude [unit]`.
func assignedValues(t *testing.T, out string, names ...string) map[string]*big.Rat {
	t.Helper()
	values := map[string]*big.Rat{}
	for _, line := range strings.Split(out, "\n") {
		for _, name := range names {
			prefix := "  " + name + " = "
			if !strings.HasPrefix(line, prefix) {
				continue
			}
			number := strings.Fields(strings.TrimPrefix(line, prefix))[0]
			r, ok := new(big.Rat).SetString(number)
			if !ok {
				t.Fatalf("%s = %q is not an exact rational:\n%s", name, number, out)
			}
			values[name] = r
		}
	}
	for _, name := range names {
		if values[name] == nil {
			t.Fatalf("no value assigned to %s:\n%s", name, out)
		}
	}
	return values
}
