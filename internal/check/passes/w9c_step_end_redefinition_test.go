package passes

import "testing"

// A step's end implicitly redefines the end at its position in the step's
// behavior (KerML 1.1 §8.3.3.3.4 checkFeatureEndRedefinition), so a feature in
// its body that redefines a member of that end is neither unresolved nor a
// duplicate of an inherited name, with or without an explicit `redefines`.
func TestW9CStepEndImplicitRedefinitionIsNoNameConflict(t *testing.T) {
	for name, ends := range map[string]string{
		"implicit": `end feature source { feature redefines sourceOutput; }
				end feature target;`,
		"explicit": `end feature source redefines TransferBefore::source { feature redefines sourceOutput; }
				end feature target redefines TransferBefore::target;`,
	} {
		src := `package Computed {
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
			` + ends + `
		}
	}
}`
		for _, code := range []string{"name-conflict", "unresolved"} {
			if got := only(kermlLibraryDiags(t, src), code); len(got) != 0 {
				t.Errorf("%s: %s: got %+v, want none", name, code, got)
			}
		}
	}
}
