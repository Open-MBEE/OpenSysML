package symbols

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

func wantNamed(t *testing.T, idx *Index, fqn string, want int) {
	t.Helper()
	if got := len(idx.LookupQualified(fqn)); got != want {
		t.Errorf("LookupQualified(%s) = %d symbols, want %d", fqn, got, want)
	}
}

// An import takes its target's public memberships, an `import all` every one,
// and what either took has the import's visibility onward (KerML 8.3.2.4.5).
func TestExpandWildcardImportsTakesPrivateMembersOnlyUnderImportAll(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "base.sysml", "package Base { private part def Hidden; part def Shown; }")
	addDoc(t, idx, "mid.sysml", "package Mid { private import Base::*; private part def Own; }")
	addDoc(t, idx, "all.sysml", "package All { public import all Mid::*; }")
	addDoc(t, idx, "plain.sysml", "package Plain { public import Mid::*; }")
	addDoc(t, idx, "onward.sysml", "package Onward { public import All::*; }")
	idx.ExpandWildcardImports()

	wantNamed(t, idx, "All::Own", 1)
	wantNamed(t, idx, "All::Shown", 1)
	wantNamed(t, idx, "All::Hidden", 0)
	wantNamed(t, idx, "Plain::Own", 0)
	wantNamed(t, idx, "Plain::Shown", 0)
	wantNamed(t, idx, "Onward::Own", 1)
	wantNamed(t, idx, "Onward::Shown", 1)
	wantNamed(t, idx, "Onward::Hidden", 0)
}

// A member whose visibility an edit changed is re-derived in its importers.
func TestExpandWildcardImportsFollowsAVisibilityEdit(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "lib.sysml", "package Lib { private part def P; }")
	addDoc(t, idx, "app.sysml", "package App { public import Lib::*; }")
	idx.ExpandWildcardImports()
	wantNamed(t, idx, "App::P", 0)

	addDoc(t, idx, "lib.sysml", "package Lib { public part def P; }")
	idx.ExpandWildcardImports()
	wantNamed(t, idx, "App::P", 1)

	addDoc(t, idx, "lib.sysml", "package Lib { private part def P; }")
	idx.ExpandWildcardImports()
	wantNamed(t, idx, "App::P", 0)
}

// An import edited between ordinary and `import all` re-derives its importer.
func TestExpandWildcardImportsFollowsAnImportAllEdit(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "lib.sysml", "package Lib { private part def P; }")
	addDoc(t, idx, "app.sysml", "package App { public import Lib::*; }")
	idx.ExpandWildcardImports()
	wantNamed(t, idx, "App::P", 0)

	addDoc(t, idx, "app.sysml", "package App { public import all Lib::*; }")
	idx.ExpandWildcardImports()
	wantNamed(t, idx, "App::P", 1)

	addDoc(t, idx, "app.sysml", "package App { public import Lib::*; }")
	idx.ExpandWildcardImports()
	wantNamed(t, idx, "App::P", 0)
}

// The snapshot stream carries an import's `all`, with what it re-exported.
func TestSnapshotKeepsImportAll(t *testing.T) {
	restored := roundTrip(t, frozenBase(t, map[string]string{
		"lib.sysml": "package Lib { private part def P; }",
		"app.sysml": "package App { public import all Lib::*; }",
	}))
	if got := restored.WildcardImportsOf("App"); len(got) != 1 || !got[0].All {
		t.Fatalf("restored WildcardImportsOf(App) = %+v, want one `import all`", got)
	}
	wantNamed(t, restored, "App::P", 1)
	if purge, derive := restored.importersToRefresh(nil, false); len(purge) != 0 || len(derive) != 0 {
		t.Errorf("restored index re-derives %v and purges %v, want its settled re-exports kept", derive, purge)
	}
}

var importCycleDocs = map[string]string{
	"s.sysml": "package S { part def A; }",
	"x.sysml": "package X { private import S::*; public import Y::*; }",
	"y.sysml": "package Y { public import all X::*; }",
}

// wantCycleRoutes checks the routes of importCycleDocs: A reaches X privately
// and Y through X, and does not come back to X by Y's import of it.
func wantCycleRoutes(t *testing.T, idx *Index) {
	t.Helper()
	a := lookupOne(t, idx, "S::A")
	if routes := idx.ReexportGates("", "X::A", a, ""); len(routes) != 0 {
		t.Errorf("X::A has %d routes from outside X, want none: Y's import of X is of an excluded namespace", len(routes))
	}
	if routes := idx.ReexportGates("", "X::A", a, "X"); len(routes) != 1 {
		t.Errorf("X::A has %d routes from within X, want its private import's", len(routes))
	}
	taken := func([]ElementFilter) bool { return true }
	if routed, accepted := idx.EachReexportGate("", "Y::A", a, "", nil, taken); !routed || !accepted {
		t.Errorf("Y::A is routed %v and accepted %v with nothing excluded, want both", routed, accepted)
	}
	// A lookup reading X's imported memberships does not take a route through X.
	if routed, accepted := idx.EachReexportGate("", "Y::A", a, "", []string{"X"}, taken); !routed || accepted {
		t.Errorf("Y::A is routed %v and accepted %v with X excluded, want routed and not accepted", routed, accepted)
	}
}

// A route does not continue into a namespace it came through: computing a
// namespace's imported memberships excludes it (KerML 8.3.2.4.5, 8.3.2.4.6).
func TestExpandWildcardImportsStopsARouteAtANamespaceItCameThrough(t *testing.T) {
	// Repeat: derivation order decides which importer is expanded first.
	for i := 0; i < 8; i++ {
		idx := NewIndex()
		for name, src := range importCycleDocs {
			addDoc(t, idx, name, src)
		}
		idx.ExpandWildcardImports()
		wantCycleRoutes(t, idx)
	}
}

func TestSnapshotKeepsTheNamespacesARouteCameThrough(t *testing.T) {
	wantCycleRoutes(t, roundTrip(t, frozenBase(t, importCycleDocs)))
}

// Layers of namespaces each importing both of the layer below reach a name by
// a number of paths doubling per layer, which no import cycle tells apart.
func TestExpandWildcardImportsKeepsOneRouteThroughLayeredImports(t *testing.T) {
	layers, below := layeredImports(40)
	src := "package S { part def P; }\n" + layers
	idx := NewIndex()
	addDoc(t, idx, "layers.sysml", src)
	idx.ExpandWildcardImports()

	p := lookupOne(t, idx, "S::P")
	for _, top := range below {
		if routes := idx.ReexportGates("", top+"::P", p, ""); len(routes) != 1 {
			t.Errorf("%s::P has %d routes, want the one every path amounts to", top, len(routes))
		}
		routed, accepted := idx.EachReexportGate("", top+"::P", p, "", []string{"A1"}, func([]ElementFilter) bool { return true })
		if !routed || !accepted {
			t.Errorf("%s::P is routed %v and accepted %v with A1 excluded, want both: B1 reaches it", top, routed, accepted)
		}
	}
}

// An import whose name a later import shadows keeps what it read from the
// namespace it named first, so that namespace stays one the routes came through.
func TestExpandWildcardImportsKeepsACycleThroughAShadowedTarget(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package A { package X { part def Other; } }")
	addDoc(t, idx, "x.sysml", "package X { private part def Hidden; public import Z::*; }")
	addDoc(t, idx, "z.sysml", "package Z { }")
	addDoc(t, idx, "p.sysml", "package P { public import all X::*; public import A::*; }")
	idx.ExpandWildcardImports()
	hidden := lookupOne(t, idx, "X::Hidden")
	if routes := idx.ReexportGates("", "P::Hidden", hidden, ""); len(routes) != 1 {
		t.Fatalf("P::Hidden has %d routes, want the one its import of X took", len(routes))
	}

	addDoc(t, idx, "z.sysml", "package Z { public import P::*; }")
	idx.ExpandWildcardImports()
	if routes := idx.ReexportGates("", "X::Hidden", hidden, ""); len(routes) != 0 {
		t.Errorf("X::Hidden has %d routes from outside X, want none: P read it from X, which is excluded", len(routes))
	}
	taken := func([]ElementFilter) bool { return true }
	if _, accepted := idx.EachReexportGate("", "Z::Hidden", hidden, "", []string{"X"}, taken); accepted {
		t.Error("Z::Hidden is accepted with X excluded, want not: its only route reads X")
	}
}

// layeredImports is `layers` layers of two namespaces, each importing both of
// the layer below, the lowest importing S; it names the top layer.
func layeredImports(layers int) (src string, top [2]string) {
	var b strings.Builder
	top = [2]string{"S", "S"}
	for i := 1; i <= layers; i++ {
		layer := [2]string{fmt.Sprintf("A%d", i), fmt.Sprintf("B%d", i)}
		for _, name := range layer {
			fmt.Fprintf(&b, "package %s { public import %s::*; public import %s::*; }\n", name, top[0], top[1])
		}
		top = layer
	}
	return b.String(), top
}

// An import an edit removed, or a cycle an edit broke, is no longer one a
// route can come back by: the layers above hold one route per name again.
func TestExpandWildcardImportsForgetsAnImportAnEditRemoved(t *testing.T) {
	src, top := layeredImports(12)
	idx := NewIndex()
	addDoc(t, idx, "layers.sysml", src)
	addDoc(t, idx, "s.sysml", "package S { part def P; public import T::*; }")
	addDoc(t, idx, "t.sysml", "package T { part def Q; }")
	idx.ExpandWildcardImports()

	oneRoute := func(when string) {
		t.Helper()
		p := lookupOne(t, idx, "S::P")
		for _, ns := range top {
			if routes := idx.ReexportGates("", ns+"::P", p, ""); len(routes) != 1 {
				t.Errorf("%s: %s::P has %d routes, want one", when, ns, len(routes))
			}
		}
		if cycle := idx.cyclic.at("S"); len(cycle) != 0 {
			t.Errorf("%s: S is on an import cycle with %v, want none", when, cycle)
		}
	}
	oneRoute("at first")

	// The dependency turns round: S no longer imports T, which imports the top.
	addDoc(t, idx, "s.sysml", "package S { part def P; }")
	addDoc(t, idx, "t.sysml", fmt.Sprintf("package T { part def Q; public import %s::*; }", top[0]))
	idx.ExpandWildcardImports()
	oneRoute("with the import reversed")

	// A cycle through every layer, and then none again.
	addDoc(t, idx, "s.sysml", "package S { part def P; public import T::*; }")
	idx.ExpandWildcardImports()
	if cycle := idx.cyclic.at("S"); len(cycle) != 25 {
		t.Errorf("S is on an import cycle of %d namespaces, want S, T and the 23 between them", len(cycle))
	}
	addDoc(t, idx, "s.sysml", "package S { part def P; }")
	idx.ExpandWildcardImports()
	oneRoute("with the cycle broken")
}

// Dropping the routes of a cycle through a namespace an import names by an
// imported name drops that name for a while, which breaks no cycle: it settles.
func TestExpandWildcardImportsSettlesACycleThroughAnImportedName(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "l.sysml", "package L { package N { part def P; public import A::*; } }")
	addDoc(t, idx, "m.sysml", "package M { public import L::*; public import A::*; }")
	addDoc(t, idx, "a.sysml", "package A { public import M::*; public import N::*; }")

	settled := make(chan struct{})
	go func() {
		defer close(settled)
		idx.ExpandWildcardImports()
	}()
	select {
	case <-settled:
	case <-time.After(30 * time.Second):
		t.Fatal("expansion does not settle")
	}

	want := []string{"A", "L::N", "M"}
	for _, ns := range want {
		if cycle := idx.cyclic.at(ns); !slices.Equal(cycle, want) {
			t.Errorf("%s is on an import cycle with %v, want %v", ns, cycle, want)
		}
	}
	p := lookupOne(t, idx, "L::N::P")
	if routes := idx.ReexportGates("", "A::P", p, ""); len(routes) == 0 {
		t.Error("A::P is not re-exported, want it read through N")
	}

	// The cycle through N goes with the import that named it.
	addDoc(t, idx, "a.sysml", "package A { public import M::*; }")
	idx.ExpandWildcardImports()
	if cycle := idx.cyclic.at("L::N"); len(cycle) != 0 {
		t.Errorf("L::N is on an import cycle with %v, want none", cycle)
	}
	if cycle, want := idx.cyclic.at("A"), []string{"A", "M"}; !slices.Equal(cycle, want) {
		t.Errorf("A is on an import cycle with %v, want %v", cycle, want)
	}
}

// Layers of public imports on one cycle keep one route per name, not one per
// path, and so does a member added below them as an edit breaks the cycle.
func TestExpandWildcardImportsKeepsOneRouteThroughLayeredImportsOnACycle(t *testing.T) {
	src, top := layeredImports(40)
	idx := NewIndex()
	addDoc(t, idx, "layers.sysml", src)
	addDoc(t, idx, "s.sysml", fmt.Sprintf("package S { part def P; public import %s::*; }", top[0]))

	expand := func() {
		t.Helper()
		settled := make(chan struct{})
		go func() {
			defer close(settled)
			idx.ExpandWildcardImports()
		}()
		select {
		case <-settled:
		case <-time.After(30 * time.Second):
			t.Fatal("expansion does not settle")
		}
		for _, key := range idx.reexportDocs.keys() {
			for doc, claim := range idx.reexportDocs.at(key) {
				if len(claim.routes) > 1 {
					t.Fatalf("%s in %s holds %d routes, want one", key.fqn, doc, len(claim.routes))
				}
			}
		}
	}
	expand()
	if cycle := idx.cyclic.at("S"); len(cycle) != 80 {
		t.Errorf("S is on an import cycle of %d namespaces, want S and the 79 it reads through", len(cycle))
	}
	p := lookupOne(t, idx, "S::P")
	accepts := func([]ElementFilter) bool { return true }
	if _, accepted := idx.EachReexportGate("", top[1]+"::P", p, "", []string{"A1"}, accepts); !accepted {
		t.Errorf("%s::P is not accepted with A1 excluded, want it read through B1", top[1])
	}
	if _, accepted := idx.EachReexportGate("", top[1]+"::P", p, "", []string{"S"}, accepts); accepted {
		t.Errorf("%s::P is accepted with S excluded, want not: S declares it", top[1])
	}

	addDoc(t, idx, "s.sysml", "package S { part def P; part def Q; }")
	expand()
	if cycle := idx.cyclic.at("S"); len(cycle) != 0 {
		t.Errorf("S is on an import cycle with %v, want none", cycle)
	}
	q := lookupOne(t, idx, "S::Q")
	for _, ns := range top {
		if routes := idx.ReexportGates("", ns+"::Q", q, ""); len(routes) != 1 {
			t.Errorf("%s::Q has %d routes, want one", ns, len(routes))
		}
	}
}

// An import can name a namespace that a later import of the same namespace
// brings in: it reads nothing until the name resolves, and is read once it does.
func TestExpandWildcardImportsReadsAnImportALaterOneResolves(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "p.sysml", "package P { package N { part def K; } }")
	addDoc(t, idx, "q.sysml", "package Q { public import N::*; public import P::*; }")
	idx.ExpandWildcardImports()

	k := lookupOne(t, idx, "P::N::K")
	if !slices.Contains(idx.reexportedAt("Q::K"), k) {
		t.Fatal("Q::K is not re-exported, want it read through the N that P supplies")
	}
	accepts := func([]ElementFilter) bool { return true }
	if _, accepted := idx.EachReexportGate("", "Q::K", k, "", []string{"P::N"}, accepts); accepted {
		t.Error("Q::K is accepted with P::N excluded, want not: it is read from there")
	}

	// The name goes with the import that supplied it.
	addDoc(t, idx, "q.sysml", "package Q { public import N::*; }")
	idx.ExpandWildcardImports()
	if got := idx.reexportedAt("Q::K"); len(got) != 0 {
		t.Errorf("Q::K is still re-exported as %v, want nothing: N no longer resolves", got)
	}
}

// A lookup excluding namespaces reads each layer of public imports on a cycle
// once, not once per path, though every read is cut at the namespace read.
func TestEachReexportGateReadsLayeredImportsOnACycleOnce(t *testing.T) {
	src, top := layeredImports(40)
	idx := NewIndex()
	addDoc(t, idx, "layers.sysml", src)
	addDoc(t, idx, "s.sysml", fmt.Sprintf("package S { public import D::*; public import %s::*; }", top[0]))
	addDoc(t, idx, "d.sysml", "package D { part def P; }")
	idx.ExpandWildcardImports()
	p := lookupOne(t, idx, "D::P")
	taken := func([]ElementFilter) bool { return true }
	for _, c := range []struct {
		excluded []string
		accepted bool
	}{{[]string{"D"}, false}, {[]string{"S"}, false}, {[]string{"A1"}, true}, {[]string{"A1", "B1"}, false}, {[]string{"B7"}, true}} {
		done := make(chan bool, 1)
		go func() {
			_, accepted := idx.EachReexportGate("", top[0]+"::P", p, "", c.excluded, taken)
			done <- accepted
		}()
		select {
		case accepted := <-done:
			if accepted != c.accepted {
				t.Errorf("%s::P excluding %v: accepted=%t, want %t", top[0], c.excluded, accepted, c.accepted)
			}
		case <-time.After(30 * time.Second):
			t.Fatalf("%s::P excluding %v: the lookup did not finish", top[0], c.excluded)
		}
	}
}

// A later import bringing in a namespace of the name an earlier import read by
// leaves the earlier import as read: expansion settles without deriving anew.
func TestExpandWildcardImportsSettlesWhenALaterImportShadowsATarget(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "u.sysml", "package U { part def M; } package V { public import U::*; }")
	idx.ExpandWildcardImports()
	claim := func() *reexportClaim {
		return idx.reexportDocs.at(reexportKey{fqn: "V::M", sym: lookupOne(t, idx, "U::M")})["u.sysml"]
	}
	before := claim()
	addDoc(t, idx, "n.sysml", "package N { part def K; }")
	addDoc(t, idx, "q.sysml", "package Q { package N { part def L; } }")
	addDoc(t, idx, "p.sysml", "package P { public import N::*; public import Q::*; }")
	idx.ExpandWildcardImports()
	lookupOne(t, idx, "P::K") // the member of the namespace the import read
	if purge, derive := idx.importersToRefresh(nil, false); len(purge)+len(derive) != 0 {
		t.Errorf("importers still to refresh once settled: purge %v, derive %v", purge, derive)
	}
	if after := claim(); before == nil || before != after {
		t.Errorf("an unrelated import was derived again: claim %p, then %p", before, after)
	}
}
