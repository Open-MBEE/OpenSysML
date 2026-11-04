package resolve_test

import "testing"

// A member a type imports under a name it inherits is hidden by the inherited
// membership, as it would be by an owned one (KerML §7.3.2, §7.2.5.4): the
// type keeps one membership of the name, so there is nothing to report.
func TestImportedNameCollidingWithInheritedNameIsHidden(t *testing.T) {
	for name, src := range map[string]string{
		"a wildcard import of a definition": `package A { part def Base { part def X; } }
package Q { part def X; }
package C { part def Sub :> A::Base { private import Q::*; } }`,
		"an explicit import of a definition": `package A { part def Base { part def X; } }
package Q { part def X; }
package C { part def Sub :> A::Base { private import Q::X; } }`,
		"a wildcard import of a usage": `package A { part def Base { part x; } }
package Q { part x; }
package C { part def Sub :> A::Base { private import Q::*; } }`,
	} {
		t.Run(name, func(t *testing.T) {
			r, _, _ := resolvedDoc(t, src)
			if len(r.Diagnostics) != 0 {
				t.Fatalf("got diagnostics %v, want none", r.Diagnostics)
			}
		})
	}
}

// Importing what a supertype's member is a redefinition of is no collision:
// the redefined feature is no longer inherited (SysML v2 §7.6.1).
func TestImportedNameMatchingARedefinedInheritedNameIsDistinguishable(t *testing.T) {
	const src = `package A { part def Base { part x; } }
package Q { part x; }
package C { part def Sub :> A::Base { private import Q::*; part y :>> x; } }`
	r, _, _ := resolvedDoc(t, src)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("got diagnostics %v, want none", r.Diagnostics)
	}
}
