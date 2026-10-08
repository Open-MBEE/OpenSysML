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
		// An alias under another name does not stand in for its target.
		{"a.sysml", "package A { part def Engine; }\npackage B { alias Spare for A::Engine; }\npackage C { part def Engine; }\npackage Use { private import B::*; private import A::*; private import C::*; }",
			[]string{"4: Duplicate of imported member name 'Engine': A::Engine (import A::*), C::Engine (import C::*)"}},
		// Two types of one name are two namespaces; their nested members collide.
		{"a.sysml", "package A { part def P { part x; } part def P { part x; } }\npackage Use { private import A::**; }",
			[]string{"1: Duplicate of other owned member name", "1: Duplicate of other owned member name",
				"2: Duplicate of imported member name 'x': A::P::x (import A::**), A::P::x (import A::**)"}},
		// An imported member repeating one the type inherits: both are the
		// type's memberships. Resolution keeps the inherited one.
		{"a.sysml", "package A { part x; }\npart def Base { part x; }\npart def Child :> Base { private import A::*; }",
			[]string{"3: Duplicate of imported member name 'x': A::x (import A::*), Base::x (inherited)"}},
		// An owned redefinition hides both the inherited and the imported name.
		{"a.sysml", "package A { part x; }\npart def Base { part x; }\npart def Child :> Base { private import A::*; part x :>> x; }", nil},
		// Metaclasses that conform in neither direction are distinguishable here too.
		{"a.sysml", "package A { attribute x; }\npart def Base { part x; }\npart def Child :> Base { private import A::*; }", nil},
	} {
		tc.run(t)
	}
}
