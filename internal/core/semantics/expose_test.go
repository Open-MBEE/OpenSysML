package semantics

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// exposedNames is the local names of what view exposes, sorted so a test states
// a set rather than the order two routes happen to reach it in.
func exposedNames(t *testing.T, m *Model, view *symbols.Symbol) []string {
	t.Helper()
	elems, err := m.ExposedElements(view)
	if err != nil {
		t.Fatalf("ExposedElements(%s): unexpected error %v", view.Name, err)
	}
	return sortedNames(elems)
}

func sortedNames(elems []*symbols.Symbol) []string {
	names := make([]string, 0, len(elems))
	for _, elem := range elems {
		name := elem.Name
		if i := strings.LastIndex(name, "::"); i >= 0 {
			name = name[i+len("::"):]
		}
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func wantNames(t *testing.T, what string, got, expect []string) {
	t.Helper()
	if !slices.Equal(got, expect) {
		t.Fatalf("%s = %v, want %v", what, got, expect)
	}
}

// A view exposing a package wildcard exposes that package's members, private
// ones included: an Expose imports all elements (SysML v2 8.3.26.2).
func TestExposedElementsNamespaceWildcard(t *testing.T) {
	m, root := buildModel(t, `
		package Lib { public part def A; private part def B; }
		view v { expose Lib::*; }
	`)
	wantNames(t, "exposed set of v", exposedNames(t, m, sym(t, root, "v")), []string{"A", "B"})
}

// `expose Lib::**` is a recursive MembershipExpose (SysML v2 8.3.26.3), so it
// exposes Lib itself and every element nested under it.
func TestExposedElementsRecursiveExpose(t *testing.T) {
	m, root := buildModel(t, `
		package Lib { part def A { part def Deep; } }
		view v { expose Lib::**; }
	`)
	wantNames(t, "exposed set of v", exposedNames(t, m, sym(t, root, "v")), []string{"A", "Deep", "Lib"})
}

// An element filter on the expose restricts what it exposes, exactly as it
// restricts what resolves through it.
func TestExposedElementsWithAnElementFilter(t *testing.T) {
	m, root := buildModel(t, `
		metadata def Safety;
		package Lib { #Safety part def A; part def B; }
		view v { expose Lib::*[@Safety]; }
	`)
	wantNames(t, "exposed set of v", exposedNames(t, m, sym(t, root, "v")), []string{"A"})
}

// A `filter` member of the view body restricts every expose it declares.
func TestExposedElementsWithAViewBodyFilter(t *testing.T) {
	m, root := buildModel(t, `
		metadata def Safety;
		package Lib { #Safety part def A; part def B; }
		package More { #Safety part def C; part def D; }
		view v { filter @Safety; expose Lib::*; expose More::*; }
	`)
	wantNames(t, "exposed set of v", exposedNames(t, m, sym(t, root, "v")), []string{"A", "C"})
}

// A view exposing another view exposes that view element itself; the exposed set
// of the exposed view stays its own.
func TestExposedElementsExposingAnotherView(t *testing.T) {
	m, root := buildModel(t, `
		package Lib { part def A; }
		view w { expose Lib::*; }
		view v { expose w; }
	`)
	wantNames(t, "exposed set of v", exposedNames(t, m, sym(t, root, "v")), []string{"w"})
	wantNames(t, "exposed set of w", exposedNames(t, m, sym(t, root, "w")), []string{"A"})
}

// A nested view exposes its own elements, and NestedViews walks them: the outer
// view's set is what its own exposes import.
func TestExposedElementsOfNestedViews(t *testing.T) {
	m, root := buildModel(t, `
		package Lib { part def A; }
		package More { part def C; }
		view outer {
			expose Lib::*;
			view inner { expose More::*; }
		}
	`)
	outer := sym(t, root, "outer")
	wantNames(t, "exposed set of outer", exposedNames(t, m, outer), []string{"A"})

	nested, err := m.NestedViews(outer)
	if err != nil {
		t.Fatalf("NestedViews(outer): unexpected error %v", err)
	}
	wantNames(t, "views nested in outer", sortedNames(nested), []string{"inner"})
	wantNames(t, "exposed set of inner", exposedNames(t, m, nested[0]), []string{"C"})
}

// A view usage typed by a view definition exposes what the definition exposes:
// an Expose is protected, so specialization carries it.
func TestExposedElementsInheritedFromAViewDefinition(t *testing.T) {
	m, root := buildModel(t, `
		package Lib { part def A; }
		package More { part def C; }
		view def V { expose Lib::*; }
		view v : V { expose More::*; }
	`)
	wantNames(t, "exposed set of v", exposedNames(t, m, sym(t, root, "v")), []string{"A", "C"})
}

// A view usage's exposes satisfy the view conditions of its definition, and of
// what that definition specializes in turn (SysML v2 7.24.2); the definition's
// own exposes are filtered the same way.
func TestExposedElementsSatisfyInheritedViewConditions(t *testing.T) {
	m, root := buildModel(t, `
		metadata def Safety;
		metadata def Reviewed;
		package Lib { #Safety part def A; part def B; #Safety #Reviewed part def C; }
		package More { #Safety #Reviewed part def D; part def E; }
		view def Safe { filter @Safety; }
		view def SafeReviewed :> Safe { filter @Reviewed; expose More::*; }
		view s : Safe { expose Lib::*; }
		view sr : SafeReviewed { expose Lib::*; }
		view again :> sr;
	`)
	wantNames(t, "exposed set of s", exposedNames(t, m, sym(t, root, "s")), []string{"A", "C"})
	wantNames(t, "exposed set of sr", exposedNames(t, m, sym(t, root, "sr")), []string{"C", "D"})
	wantNames(t, "exposed set of again", exposedNames(t, m, sym(t, root, "again")), []string{"C", "D"})
}

// An expose a view inherits is admitted against the inheriting view's own
// conditions: the definition's set stays its own, the narrower usage's shrinks.
func TestExposedElementsInheritedExposeSatisfiesDerivedConditions(t *testing.T) {
	m, root := buildModel(t, `
		metadata def Safety;
		metadata def Reviewed;
		package Lib { #Safety part def A; part def B; #Safety #Reviewed part def C; }
		view def Safe { filter @Safety; expose Lib::*; }
		view sr : Safe { filter @Reviewed; }
		view srr :> sr;
	`)
	wantNames(t, "exposed set of Safe", exposedNames(t, m, sym(t, root, "Safe")), []string{"A", "C"})
	wantNames(t, "exposed set of sr", exposedNames(t, m, sym(t, root, "sr")), []string{"C"})
	wantNames(t, "exposed set of srr", exposedNames(t, m, sym(t, root, "srr")), []string{"C"})

	// Name lookup in the narrower view agrees with its exposed set.
	sr := sym(t, root, "sr")
	for _, name := range []string{"A", "B"} {
		if _, ok := m.resolver.ResolveName(sr.Scope, name, &ast.QualifiedName{Parts: []ast.NameSegment{{Text: name}}}); ok {
			t.Errorf("%s resolves in sr, which its filter rejects", name)
		}
	}
	if _, ok := m.resolver.ResolveName(sr.Scope, "C", &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "C"}}}); !ok {
		t.Error("C does not resolve in sr, which its inherited expose admits")
	}
}

// A view exposing nothing has an empty exposed set, which is no error.
func TestExposedElementsOfAViewExposingNothing(t *testing.T) {
	m, root := buildModel(t, `
		package Lib { part def A; }
		view v { import Lib::*; }
	`)
	elems, err := m.ExposedElements(sym(t, root, "v"))
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if len(elems) != 0 {
		t.Fatalf("a view declaring no expose exposes %v, want nothing (a plain import exposes nothing)", sortedNames(elems))
	}
}

// The exposed set of an element that is no view is a typed error, so a caller
// can tell "exposes nothing" from "cannot expose".
func TestExposedElementsOfANonView(t *testing.T) {
	m, root := buildModel(t, "package Lib { part def A; } part p;")
	for _, name := range []string{"Lib", "p"} {
		if _, err := m.ExposedElements(sym(t, root, name)); !errors.Is(err, ErrNotAView) {
			t.Errorf("ExposedElements(%s) error = %v, want ErrNotAView", name, err)
		}
	}
	if _, err := m.ExposedElements(nil); !errors.Is(err, ErrNotAView) {
		t.Errorf("ExposedElements(nil) error = %v, want ErrNotAView", err)
	}
	if _, err := m.NestedViews(nil); !errors.Is(err, ErrNotAView) {
		t.Errorf("NestedViews(nil) error = %v, want ErrNotAView", err)
	}
}

// A filtered recursive expose reaches an annotated element through namespaces
// the filter itself rejects, as a lookup through the same expose does.
func TestExposedElementsRecursiveExposeWithAFilterReachesNestedElements(t *testing.T) {
	m, root := buildModel(t, `
		metadata def Safety;
		package Lib { package Inner { #Safety part def A; part def B; } }
		view v { expose Lib::**[@Safety]; }
	`)
	wantNames(t, "exposed set of v", exposedNames(t, m, sym(t, root, "v")), []string{"A"})
}

// A nested namespace sharing a top-level namespace's short name exposes its own
// members: the walk keys the index by the qualified name, not the short one.
func TestExposedElementsRecursiveExposeIntoANamespaceWithAReusedName(t *testing.T) {
	m, root := buildModel(t, `
		package Lib { package Inner { part def X; } }
		package Inner { part def Y; }
		view v { expose Lib::**; }
	`)
	wantNames(t, "exposed set of v", exposedNames(t, m, sym(t, root, "v")), []string{"Inner", "Lib", "X"})
}
