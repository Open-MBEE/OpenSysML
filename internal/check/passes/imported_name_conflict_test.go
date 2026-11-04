package passes

import "testing"

// Imported memberships two imports make indistinguishable are hidden from the
// importing namespace (KerML 7.2.5.4), which is a derivation and no violation:
// nothing is reported for them, whether the collision is with another import,
// with an inherited member or through an expose. Only owned names repeat.
func TestImportedNameConflicts(t *testing.T) {
	for _, tc := range []metaclassCase{
		{"a.sysml", "package A { part def Engine; }\npackage B { part def Engine; }\npackage C { private import A::*; private import B::*; }", nil},
		{"a.sysml", "package A { part def Engine; }\npackage B { item def Engine; }\npackage C { private import A::*; private import B::*; }", nil},
		{"a.sysml", "package A { part def Engine; }\npackage B { attribute def Engine; }\npackage C { private import A::*; private import B::*; }", nil},
		{"a.kerml", "package A { class Engine; }\npackage B { class Engine; }\npackage C { private import A::*; private import B::*; }", nil},
		{"a.sysml", "package A { attribute def m; part def Engine; }\npackage C {\nprivate import ISQ::*;\nprivate import SI::*;\nprivate import ScalarValues::*;\nprivate import Quantities::*;\nprivate import A::*;\n}", nil},
		{"a.sysml", "package V { part def P { attribute mass; } part v { attribute mass; } }\npackage C { private import V::**; }", nil},
		// Two types of one name are two namespaces; their nested members
		// collide at the importer and are hidden there, the owned repeat reported.
		{"a.sysml", "package A { part def P { part x; } part def P { part x; } }\npackage Use { private import A::**; }",
			[]string{"1: Duplicate of other owned member name", "1: Duplicate of other owned member name"}},
		// An imported member repeating one the type inherits is hidden by it (KerML 7.3.2).
		{"a.sysml", "package A { part x; }\npart def Base { part x; }\npart def Child :> Base { private import A::*; }", nil},
		{"a.sysml", "package A { part x; }\npart def Base { part x; }\npart def Child :> Base { private import A::*; part x :>> x; }", nil},
		{"a.sysml", "package V {\npart a { part p; }\npart b { part p; }\nview v { expose a::**; expose b::**; }\n}", nil},
		{"a.sysml", "package V {\npart a { part p; }\npart b { part p; }\nview v { private import a::*; expose b::*; }\n}", nil},
	} {
		tc.run(t)
	}
}
