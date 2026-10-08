package passes

import "testing"

// Imported memberships two imports make indistinguishable warn once per name
// on the import bringing the later membership, with the standard library
// loaded so metaclasses are known: a pair whose metaclasses conform in neither
// direction is distinguishable (KerML 8.3.2.4.3), library members never take
// part, and names are spelt as the notation quotes them.
func TestImportedNameConflicts(t *testing.T) {
	const engine = "3: Duplicate of imported member name 'Engine': A::Engine (import A::*), B::Engine (import B::*)"
	for _, tc := range []metaclassCase{
		{"a.sysml", "package A { part def Engine; }\npackage B { part def Engine; }\npackage C { private import A::*; private import B::*; part e : Engine; }",
			[]string{engine}},
		{"a.sysml", "package A { part def Engine; }\npackage B { item def Engine; }\npackage C { private import A::*; private import B::*; }",
			[]string{engine}},
		{"a.sysml", "package A { part def Engine; }\npackage B { attribute def Engine; }\npackage C { private import A::*; private import B::*; }", nil},
		{"a.kerml", "package A { class Engine; }\npackage B { datatype Engine; }\npackage C { private import A::*; private import B::*; }", nil},
		{"a.kerml", "package A { class Engine; }\npackage B { class Engine; }\npackage C { private import A::*; private import B::*; }",
			[]string{engine}},
		// Library content: the root packages every model sees, the usual
		// wildcard imports, and a model name a library member repeats.
		{"a.sysml", "package A { attribute def m; part def Engine; }\npackage C {\nprivate import ISQ::*;\nprivate import SI::*;\nprivate import ScalarValues::*;\nprivate import Quantities::*;\nprivate import A::*;\n}", nil},
		// Two memberships of one element — an alias and the element — do not collide.
		{"a.sysml", "package A { part def Engine; }\npackage B { alias Engine for A::Engine; }\npackage C { private import A::*; private import B::*; }", nil},
		{"a.sysml", "package 'Parts A' { part def Engine; }\npackage 'Parts B' { part def Engine; }\npackage C { private import 'Parts A'::*; private import 'Parts B'::*; }",
			[]string{"3: Duplicate of imported member name 'Engine': 'Parts A'::Engine (import 'Parts A'::*), 'Parts B'::Engine (import 'Parts B'::*)"}},
		// A recursive import's nested members take part, type bodies included.
		{"a.sysml", "package V { part def P { attribute mass; } part v { attribute mass; } }\npackage C { private import V::**; }",
			[]string{"2: Duplicate of imported member name 'mass': V::P::mass (import V::**), V::v::mass (import V::**)"}},
	} {
		tc.run(t)
	}
}
