package model_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// importCycleShape is a model whose imports form cycles, as packages, with the unresolved
// references it has: the line each is written on and the name.
type importCycleShape struct {
	name     string
	packages []string
	// anyOrder holds where the answer does not depend on the order the packages are written in.
	anyOrder   bool
	unresolved []string
}

// importCycleShapes are cycles of imports mixing `all`, recursive, filtered and membership imports
// with names declared twice, each with the answer KerML 8.3.2.4.5 and 8.3.2.4.6 give it.
var importCycleShapes = []importCycleShape{
	{
		// A's `filter` member keeps X out of what A imports; B brings X all the same, on a route of its own.
		name: "package reached under a filter member and without one",
		packages: []string{
			"package M { metadata def Safety; }",
			"package C { part def X; }",
			"package A { public import C::*; filter @M::Safety; }",
			"package B { public import C::*; }",
			"package U { public import A::*; public import B::*; part u : X; }",
			"package V { private import U::*; part v : X; }",
		},
		anyOrder: true,
	},
	{
		// A membership import names its target in its own right: N2::B is N4::B, a member of N2 by its
		// import of N4, whatever route through the cycle is reading N1 when the import is enumerated.
		name: "membership import of a member the cycle brings back",
		packages: []string{
			"package N0 { public import N4::**; }",
			"package O0 { private import N0::*; part s_B : B; }",
			"package N1 { public import N2::B; }",
			"package N2 { public import N4::*; }",
			"package N4 { public attribute def B; public import N1::*; }",
		},
		anyOrder: true,
	},
	{
		// N3::A and N4::A both resolve: N3 reads N0, whose `import all` reads N2, whose `import all` takes N4's
		// private A (8.3.2.4.5 visibleMemberships with includeAll), and N4 takes A back through N1.
		name: "private member taken by import all through the cycle",
		packages: []string{
			"package N0 { public import all N2::*; }",
			"package O0 { private import N0::*; part s_B : B; }",
			"package N1 { public import N3::A; public import N4::A; }",
			"package N2 { private import all N4::*; }",
			"package N3 { public import N0::*; }",
			"package N4 { private attribute def A; public import N1::*; }",
		},
		anyOrder: true,
		unresolved: []string{
			"part s_B : B; -> B",
		},
	},
	{
		// Each filter names a member that does not exist, so a filtered import brings nothing; B is no type.
		name: "filtered and recursive imports with a hidden name",
		packages: []string{
			"package N0 { private import N1::*; public import N3::*[@N3::M3]; }",
			"package N1 { public part B; public import all N4::**; }",
			"package O1 { private import N1::*; part s_A : A; }",
			"package N3 { part B; public import all N4::*[@N4::M4]; }",
			"package O3 { private import N3::*; part s_B : B; }",
			"package N4 { public import N0::*; private import N1::*; public import all N3::*[@N3::M3]; }",
		},
		anyOrder: true,
		unresolved: []string{
			"part s_A : A; -> A",
			"part s_B : B; -> B",
			"public import N3::*[@N3::M3]; -> N3::M3",
			"public import all N3::*[@N3::M3]; -> N3::M3",
			"public import all N4::*[@N4::M4]; -> N4::M4",
		},
	},
	{
		// B is hidden in O0 by N0::B and N1::B. Whether C is hidden in O1 depends on whether a namespace in the
		// cycle that hides a name passes it on, which the operations leave open: the answer in this order is kept.
		name: "hiding inside the cycle, recursive imports",
		packages: []string{
			"package N0 { part def B; attribute def C; public import N1::*; }",
			"package O0 { private import N0::*; part s_B : B; }",
			"package N1 { part def B; public import N2::*; private import N3::**; }",
			"package O1 { private import N1::*; part s_C : C; }",
			"package N2 { public part def C; public import all N4::*[@N4::M4]; }",
			"package N3 { public import N2::**; public import N4::**; }",
			"package N4 { public attribute def C; package Inner { part def C; } private import N0::*; private import N3::*; }",
		},
		anyOrder: false,
		unresolved: []string{
			"part s_B : B; -> B",
			"public import all N4::*[@N4::M4]; -> N4::M4",
		},
	},
	{
		// N1::A and N2::A reach N0 and O2 by two imports each and hide one another (7.2.5.4).
		name: "two definitions of one name hidden around the cycle",
		packages: []string{
			"package N0 { private import N1::*[@N1::M1]; public import all N4::*; part q_A : A; }",
			"package N1 { part def A; public import N2::**; public import N4::*[@N4::M4]; }",
			"package N2 { part def A; public import N0::*; }",
			"package O2 { private import N2::*; part s_A : A; }",
			"package N3 { public import N1::*; }",
			"package N4 { public import N1::*; public import N2::*; private import all N3::*[@N3::M3]; }",
		},
		anyOrder: true,
		unresolved: []string{
			"part q_A : A; -> A",
			"part s_A : A; -> A",
			"private import N1::*[@N1::M1]; -> N1::M1",
			"private import all N3::*[@N3::M3]; -> N3::M3",
			"public import N4::*[@N4::M4]; -> N4::M4",
		},
	},
	{
		// Whether N2, which hides C, passes N0::C on to N4's importers is left open by the operations:
		// the answer in this order is kept.
		name: "hiding inside the cycle, filtered imports",
		packages: []string{
			"package N0 { part def C; }",
			"package N1 { private import N4::*[@N4::M4]; part q_C : C; }",
			"package N2 { public import N0::*; public import N4::*; }",
			"package N4 { part def C; public import N1::*[@N1::M1]; public import N2::*[@N2::M2]; }",
			"package O4 { private import N4::*; part s_C : C; }",
		},
		anyOrder: false,
		unresolved: []string{
			"part q_C : C; -> C",
			"private import N4::*[@N4::M4]; -> N4::M4",
			"public import N1::*[@N1::M1]; -> N1::M1",
			"public import N2::*[@N2::M2]; -> N2::M2",
		},
	},
	{
		// Whether A is hidden where N2 brings it through the cycle is left open by the operations:
		// the answer in this order is kept.
		name: "hiding inside the cycle, import all",
		packages: []string{
			"package N0 { public part def A; private import N2::**; }",
			"package O0 { private import N0::*; part s_C : C; }",
			"package N1 { public import N0::*[@N0::M0]; public import N2::*; part q_B : B; }",
			"package N2 { part def A; part B; public import N3::**; }",
			"package O2 { private import N2::*; part s_A : A; }",
			"package N3 { public import all N1::**; }",
		},
		anyOrder: false,
		unresolved: []string{
			"part s_A : A; -> A",
			"part s_C : C; -> C",
			"public import N0::*[@N0::M0]; -> N0::M0",
		},
	},
}

// unresolvedIn analyses the packages as one document and lists its unresolved references.
func unresolvedIn(packages []string) []string {
	src := strings.Join(packages, "\n")
	ws := model.NewWorkspace()
	ws.Open("cycle.sysml", []byte(src), 1)
	var out []string
	for _, d := range ws.Diagnostics("cycle.sysml") {
		name, found := strings.CutPrefix(d.Message, "unresolved reference: ")
		if !found {
			continue
		}
		name, _, _ = strings.Cut(name, " ")
		// The member the reference is written in runs from the `{` or `;` before it to the next `;`.
		at := strings.LastIndexAny(src[:d.Span.Offset], "{;") + 2
		out = append(out, fmt.Sprintf("%s -> %s", src[at:d.Span.Offset+strings.Index(src[d.Span.Offset:], ";")+1], name))
	}
	slices.Sort(out)
	return out
}

// A cycle of imports resolves as the operations read it, and where they settle the answer it is
// the same in whatever order the packages are written.
func TestImportCycleShapesResolveAsTheOperationsReadThem(t *testing.T) {
	for _, shape := range importCycleShapes {
		t.Run(shape.name, func(t *testing.T) {
			orders := [][]string{shape.packages}
			if shape.anyOrder {
				for i := 1; i < len(shape.packages); i++ {
					orders = append(orders, append(slices.Clone(shape.packages[i:]), shape.packages[:i]...))
				}
				reversed := slices.Clone(shape.packages)
				slices.Reverse(reversed)
				orders = append(orders, reversed)
			}
			for _, order := range orders {
				if got := unresolvedIn(order); !slices.Equal(got, shape.unresolved) {
					t.Errorf("packages in the order %s\nunresolved: %q\nwant:       %q", strings.Join(order, "\n"), got, shape.unresolved)
				}
			}
		})
	}
}
