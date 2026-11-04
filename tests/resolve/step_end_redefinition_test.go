package resolve_test

import "testing"

// An end feature of a step implicitly redefines the end at the same position in
// the step's type (KerML 1.1 §8.3.3.3.4 checkFeatureEndRedefinition), so the
// members of the redefined end are visible in the step end's body.
func TestStepEndImplicitlyRedefinesTypeEnd(t *testing.T) {
	const general = `package Computed {
	class Occ;
	interaction Transfer {
		end feature source : Occ { feature sourceOutput; }
		end feature target : Occ;
	}
	interaction TransferBefore specializes Transfer {
		end feature source : Occ redefines Transfer::source;
		end feature target : Occ redefines Transfer::target;
	}
	behavior Observe {
		step transfer : TransferBefore {
`
	for name, ends := range map[string]string{
		"implicit": `			end feature source { feature redefines sourceOutput; }
			end feature target;`,
		"explicit": `			end feature source redefines TransferBefore::source { feature redefines sourceOutput; }
			end feature target redefines TransferBefore::target;`,
	} {
		r, _, _ := resolvedDocNamed(t, name+".kerml", general+ends+"\n\t\t}\n\t}\n}")
		for _, d := range r.Diagnostics {
			t.Errorf("%s: unexpected diagnostic: %s", name, d.Message)
		}
	}
}
