package resolve_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
)

// A type inherits what its import took with the namespaces before it excluded
// (KerML 8.3.2.4.5), and a requirement or case body conflicts with it as any type does.
func TestInheritedImportsOfACycleConflictByRoute(t *testing.T) {
	for _, tc := range []struct {
		imp  string
		want []string
	}{
		{"public import C1::*", []string{"helper from C2"}},
		{"public import all C1::*", []string{"helper from C2", "limit from C1"}},
		{"public import C2::*", []string{"limit from C1"}},
		{"private import all C1::*", nil},
	} {
		t.Run(tc.imp, func(t *testing.T) {
			for _, bodies := range [][2]string{
				{"requirement def G", "requirement def S :> G"},
				{"requirement def G", "requirement s : G"},
				{"use case def G", "use case s : G"},
				{"part def G", "part def S :> G"},
			} {
				r, _, _ := resolvedDoc(t, `
					package C1 { public import all C2::*; private part limit; }
					package C2 { public import all C1::*; private part helper; }
					package P {
						`+bodies[0]+` { `+tc.imp+`; }
						`+bodies[1]+` { part helper; part limit; }
					}`)
				var got []string
				for _, d := range diagnosticsWithCode(r, resolve.CodeNameConflict) {
					got = append(got, d.Message)
				}
				var want []string
				for _, name := range tc.want {
					head, from, _ := strings.Cut(name, " from ")
					want = append(want, "Duplicate of inherited member name '"+head+"' from "+from)
				}
				if !slices.Equal(got, want) {
					t.Errorf("%s: name conflicts = %q, want %q", bodies[1], got, want)
				}
			}
		})
	}
}

// A type reached through an import cycle keeps its redefinitions' masking: the
// redefined name answers only where a redefining feature takes it (KerML 7.3.4.5).
func TestRedefinedNamesThroughAnImportCycle(t *testing.T) {
	wantUnresolved(t, `
		package C1 { public import all C2::*; private part def Base { attribute a; attribute kept; } }
		package C2 {
			public import all C1::*;
			private part def Named :> Base { attribute b redefines a; }
			private part def Unnamed :> Base { attribute redefines a; }
		}
		package Use {
			private import C1::*;
			part s : Named; part t : Base;
			attribute k :> Named::kept; attribute m :> Named::a; attribute n :> Named::b;
			attribute o :> Unnamed::a; attribute p :> C2::Base::a; attribute q :> C1::Base::a;
			attribute u :> C1::Named::a; attribute v :> C1::Unnamed::a; attribute w :> C2::Unnamed::a;
		}`,
		"Base", "C1::Base::a", "C1::Named::a", "C2::Unnamed::a", "Named::a")
}
