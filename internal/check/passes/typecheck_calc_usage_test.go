package passes

import (
	"strings"
	"testing"
)

// twoOutputs is the calc every case here reads outputs of: one input and two
// outputs, one of them typed by its default rather than by a declaration.
const twoOutputs = `calc def Two {
		in n : ScalarValues::Integer;
		out a = n + 1;
		out b : ScalarValues::Integer = n * 2;
	}
	`

// A calc usage inherits the outputs of the calc it is typed by, so reading one
// through a feature chain types as that output does (SysML 7.6.6, 7.17).
func TestCalcUsageOutputTypesAsTheOutputItNames(t *testing.T) {
	wantNoDiags(t, `package M {
	`+twoOutputs+`calc c : Two { in n = 5; }
	attribute fromDefault : ScalarValues::Integer = c.a;
	attribute fromDeclaration : ScalarValues::Integer = c.b;
}`)
}

// An output typed only by its default carries that type to its readers, so
// binding it where the type cannot go is reported.
func TestCalcUsageOutputTypedByItsDefaultIsChecked(t *testing.T) {
	wantOneDiag(t, `package M {
	`+twoOutputs+`calc c : Two { in n = 5; }
	attribute wrong : ScalarValues::String = c.a;
}`, "cannot bind Integer value to a feature typed by String")
}

// A declared output type is checked the same way.
func TestCalcUsageDeclaredOutputTypeIsChecked(t *testing.T) {
	wantOneDiag(t, `package M {
	`+twoOutputs+`calc c : Two { in n = 5; }
	attribute wrong : ScalarValues::Boolean = c.b;
}`, "cannot bind Integer value to a feature typed by Boolean")
}

// The usage may sit inside a part definition and be read from a sibling
// feature's default, which is the parametric-budget pattern.
func TestCalcUsageOutputInsideAPartDefinition(t *testing.T) {
	wantNoDiags(t, `package M {
	`+twoOutputs+`part def Rig {
		attribute base : ScalarValues::Integer = 7;
		calc d : Two { in n = base; }
		attribute q : ScalarValues::Integer = d.a;
		part inner {
			calc e : Two { in n = 3; }
			attribute deep : ScalarValues::Integer = e.b;
		}
	}
}`)
}

// An output read through a chain is typed through the outputs' own values, so
// an output valued from another output types as the chain of defaults does.
func TestCalcUsageOutputFromAnotherOutput(t *testing.T) {
	wantNoDiags(t, `package M {
	calc def Chained {
		in n : ScalarValues::Integer;
		out a = n + 1;
		out b = a * 2;
	}
	calc c : Chained { in n = 5; }
	attribute z : ScalarValues::Integer = c.b;
}`)
}

// Outputs valued from each other have no type to compute, and the checker says
// nothing rather than recursing (the runtime reports the cycle).
func TestCalcUsageCyclicOutputsProduceNoTypeDiagnostic(t *testing.T) {
	wantNoDiags(t, `package M {
	calc def Knot {
		in n : ScalarValues::Integer;
		out a = b + 1;
		out b = a + n;
	}
	calc c : Knot { in n = 5; }
	attribute z : ScalarValues::Integer = c.a;
}`)
}

// A name the calc declares no output for is not a member of the usage, which
// the name-resolution tier reports; the type tier stays silent so the error is
// reported once.
func TestCalcUsageUnknownOutputIsUnresolved(t *testing.T) {
	src := scalarPrelude + `package M {
	` + twoOutputs + `calc c : Two { in n = 5; }
	attribute z : ScalarValues::Integer = c.nope;
}`
	diags := nameresDiags(t, src)
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "unresolved member: nope") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an unresolved member diagnostic for the unknown output, got %v", diags)
	}
	if typed := typeDiags(t, src); len(typed) != 0 {
		t.Fatalf("expected no type diagnostics for an unresolved output, got %v", typed)
	}
}
