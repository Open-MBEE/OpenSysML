package semantics

import (
	"fmt"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// routeModel is two chained imports with a view over each namespace; the verbs
// are how Mid imports ReviewSource and how Probe imports Mid.
const routeModel = `
	package ReviewSource { public part def Visible; private part def Hidden; }
	package Mid { %s ReviewSource::*; private part def OwnHidden; }
	package Probe {
		%s Mid::*;
		view focused { expose Mid::*; }
		part a : OwnHidden; part b : Visible; part c : Hidden;
	}
	package Probe2 { view focused2 { expose Probe::*; } }
	package Probe3 { %s Probe::*; part a : OwnHidden; part b : Visible; part c : Hidden; }
`

// routeCase builds routeModel, with the index's wildcard re-exports expanded
// first when expand is set: both routes to a membership must agree.
func routeCase(t *testing.T, expand bool, mid, probe, probe3 string) (exposed func(pkg, view string) []string, typed func(pkg string) []string) {
	t.Helper()
	src := fmt.Sprintf(routeModel, mid, probe, probe3)
	m, root, r, doc := buildUnresolvedModel(t, "t.sysml", source.KindOf("t.sysml"), src)
	if expand {
		r.Index().ExpandWildcardImports()
	}
	r.ResolveDocument("t.sysml", doc)
	exposed = func(pkg, view string) []string {
		return exposedNames(t, m, sym(t, sym(t, root, pkg).Scope, view))
	}
	typed = func(pkg string) []string {
		var out []string
		for _, name := range []string{"a", "b", "c"} {
			for _, sup := range m.DirectSupertypes(sym(t, sym(t, root, pkg).Scope, name)) {
				out = append(out, sup.Name)
			}
		}
		slices.Sort(out)
		return out
	}
	return exposed, typed
}

func eachIndexRoute(t *testing.T, f func(t *testing.T, expand bool)) {
	t.Helper()
	for _, expand := range []bool{false, true} {
		t.Run(fmt.Sprintf("expanded=%v", expand), func(t *testing.T) { f(t, expand) })
	}
}

// An Expose takes every membership of the namespace it names (SysML v2 8.3.26.2),
// and a private import there took only its target's public ones (KerML 8.3.2.4.2).
func TestExposedElementsStopAtAnIntermediateOrdinaryImport(t *testing.T) {
	eachIndexRoute(t, func(t *testing.T, expand bool) {
		exposed, _ := routeCase(t, expand, "private import", "private import", "private import")
		wantNames(t, "exposed set of focused", exposed("Probe", "focused"), []string{"OwnHidden", "Visible"})
	})
}

// The memberships Probe imported are the ones its own ordinary import admitted:
// the outer Expose does not widen it into an `import all`.
func TestExposedElementsDoNotWidenAnIntermediateImport(t *testing.T) {
	eachIndexRoute(t, func(t *testing.T, expand bool) {
		exposed, _ := routeCase(t, expand, "private import", "private import", "private import")
		wantNames(t, "exposed set of focused2", exposed("Probe2", "focused2"), []string{"a", "b", "c", "focused"})
	})
}

// `import all Mid::*` takes Mid's private member and what Mid imported privately,
// but not what Mid's own ordinary import never took.
func TestExposedElementsThroughAnIntermediateImportAll(t *testing.T) {
	eachIndexRoute(t, func(t *testing.T, expand bool) {
		exposed, typed := routeCase(t, expand, "private import", "private import all", "private import")
		wantNames(t, "exposed set of focused2", exposed("Probe2", "focused2"),
			[]string{"OwnHidden", "Visible", "a", "b", "c", "focused"})
		wantNames(t, "names Probe resolves", typed("Probe"), []string{"OwnHidden", "Visible"})
		wantNames(t, "names Probe3 resolves", typed("Probe3"), nil)
	})
}

// A public import carries its target's public memberships onward, and no others.
func TestExposedElementsThroughAPublicIntermediateImport(t *testing.T) {
	eachIndexRoute(t, func(t *testing.T, expand bool) {
		exposed, typed := routeCase(t, expand, "public import", "public import", "private import")
		wantNames(t, "exposed set of focused", exposed("Probe", "focused"), []string{"OwnHidden", "Visible"})
		wantNames(t, "exposed set of focused2", exposed("Probe2", "focused2"), []string{"Visible", "a", "b", "c", "focused"})
		wantNames(t, "names Probe resolves", typed("Probe"), []string{"Visible"})
		wantNames(t, "names Probe3 resolves", typed("Probe3"), []string{"Visible"})
	})
}

// importRoute is one import of routeModel: its visibility and whether it is `import all`.
type importRoute struct{ public, all bool }

func (i importRoute) verb() string {
	verb := "private import"
	if i.public {
		verb = "public import"
	}
	if i.all {
		verb += " all"
	}
	return verb
}

// taken is what the import takes from a namespace with the given memberships
// (KerML 8.3.2.4.5 visibleMemberships): all of them, or only the public ones.
func (i importRoute) taken(all, public []string) []string {
	if i.all {
		return all
	}
	return public
}

// carried is what of taken stays visible outside the importing namespace: an
// imported membership has the visibility of its Import (KerML 8.3.2.4.5 visibilityOf).
func (i importRoute) carried(taken []string) []string {
	if i.public {
		return taken
	}
	return nil
}

func sortedUnion(lists ...[]string) []string {
	var out []string
	for _, list := range lists {
		out = append(out, list...)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// Every visibility and `all` combination of the three imports, by both index
// routes, against KerML 8.3.2.4.5 computed directly.
func TestImportRouteMembershipsFollowEachImport(t *testing.T) {
	var routes []importRoute
	for _, public := range []bool{false, true} {
		for _, all := range []bool{false, true} {
			routes = append(routes, importRoute{public: public, all: all})
		}
	}
	own := []string{"a", "b", "c", "focused"}
	for _, mid := range routes {
		for _, probe := range routes {
			for _, probe3 := range routes {
				midImported := mid.taken([]string{"Hidden", "Visible"}, []string{"Visible"})
				midAll := sortedUnion([]string{"OwnHidden"}, midImported)
				probeImported := probe.taken(midAll, mid.carried(midImported))
				probeAll := sortedUnion(own, probeImported)
				probe3Imported := probe3.taken(probeAll, sortedUnion(own, probe.carried(probeImported)))
				name := fmt.Sprintf("mid=%s/probe=%s/probe3=%s", mid.verb(), probe.verb(), probe3.verb())
				t.Run(name, func(t *testing.T) {
					eachIndexRoute(t, func(t *testing.T, expand bool) {
						exposed, typed := routeCase(t, expand, mid.verb(), probe.verb(), probe3.verb())
						wantNames(t, "exposed set of focused", exposed("Probe", "focused"), midAll)
						wantNames(t, "exposed set of focused2", exposed("Probe2", "focused2"), probeAll)
						wantNames(t, "names Probe resolves", typed("Probe"), withoutNames(probeImported, own))
						wantNames(t, "names Probe3 resolves", typed("Probe3"), withoutNames(probe3Imported, own))
					})
				})
			}
		}
	}
}

func withoutNames(names, drop []string) []string {
	var out []string
	for _, name := range names {
		if !slices.Contains(drop, name) {
			out = append(out, name)
		}
	}
	return out
}

// An alias is a membership Mid owns, so its own visibility decides: the public
// one crosses an ordinary import, the private one only an `import all`.
func TestExposedElementsCarryAnAliasByItsOwnVisibility(t *testing.T) {
	const src = `
		package ReviewSource { public part def Visible; private part def Hidden; }
		package Mid {
			private import ReviewSource::*;
			private part def OwnHidden;
			public alias Shown for OwnHidden;
			private alias Kept for Visible;
		}
		package Ordinary { private import Mid::*; part s : Shown; part k : Kept; }
		package All { private import all Mid::*; part s : Shown; part k : Kept; }
		package Views {
			view ordinary { expose Ordinary::*; }
			view all { expose All::*; }
			view mid { expose Mid::*; }
		}
	`
	eachIndexRoute(t, func(t *testing.T, expand bool) {
		m, root, r, doc := buildUnresolvedModel(t, "t.sysml", source.KindOf("t.sysml"), src)
		if expand {
			r.Index().ExpandWildcardImports()
		}
		r.ResolveDocument("t.sysml", doc)
		views := sym(t, root, "Views").Scope
		wantNames(t, "exposed set of ordinary", exposedNames(t, m, sym(t, views, "ordinary")), []string{"Shown", "k", "s"})
		wantNames(t, "exposed set of all", exposedNames(t, m, sym(t, views, "all")),
			[]string{"Kept", "OwnHidden", "Shown", "Visible", "k", "s"})
		wantNames(t, "exposed set of mid", exposedNames(t, m, sym(t, views, "mid")),
			[]string{"Kept", "OwnHidden", "Shown", "Visible"})
		typed := func(pkg, usage string) int {
			return len(m.DirectSupertypes(sym(t, sym(t, root, pkg).Scope, usage)))
		}
		if typed("Ordinary", "s") != 1 || typed("Ordinary", "k") != 0 {
			t.Errorf("an ordinary import resolves Shown %d times and Kept %d, want 1 and 0", typed("Ordinary", "s"), typed("Ordinary", "k"))
		}
		if typed("All", "s") != 1 || typed("All", "k") != 1 {
			t.Errorf("an import all resolves Shown %d times and Kept %d, want 1 and 1", typed("All", "s"), typed("All", "k"))
		}
	})
}

// A membership import is a route like a wildcard one, and a recursive Expose
// applies each nested namespace's imports the same way.
func TestExposedElementsThroughMembershipAndRecursiveRoutes(t *testing.T) {
	const src = `
		package ReviewSource {
			public part def Visible;
			private part def Hidden;
			package Deep { part def DeepVisible; private part def DeepHidden; }
			private package Vault { part def VaultPart; }
		}
		package Named { private import ReviewSource::Visible; }
		package Recursive { private import ReviewSource::*::**; }
		package Outer { package Inner { private import ReviewSource::*; private part def InnerHidden; } }
		package Views {
			view named { expose Named::*; }
			view recursive { expose Recursive::*; }
			view outer { expose Outer::**; }
		}
	`
	eachIndexRoute(t, func(t *testing.T, expand bool) {
		m, root, r, doc := buildUnresolvedModel(t, "t.sysml", source.KindOf("t.sysml"), src)
		if expand {
			r.Index().ExpandWildcardImports()
		}
		r.ResolveDocument("t.sysml", doc)
		views := sym(t, root, "Views").Scope
		wantNames(t, "exposed set of named", exposedNames(t, m, sym(t, views, "named")), []string{"Visible"})
		wantNames(t, "exposed set of recursive", exposedNames(t, m, sym(t, views, "recursive")),
			[]string{"Deep", "DeepVisible", "Visible"})
		wantNames(t, "exposed set of outer", exposedNames(t, m, sym(t, views, "outer")),
			[]string{"Deep", "Inner", "InnerHidden", "Outer", "Visible"})
	})
}

// Two declarations of one namespace each contribute their imports, which only the
// expanded index joins; the private one still takes only public memberships.
func TestExposedElementsOfANamespaceDeclaredTwice(t *testing.T) {
	const src = `
		package SourceA { part def A; private part def AHidden; }
		package SourceB { part def B; private part def BHidden; }
		package Split { public import SourceA::*; }
		package Split { private import SourceB::*; }
		view v { expose Split::*; }
	`
	m, root, r, doc := buildUnresolvedModel(t, "t.sysml", source.KindOf("t.sysml"), src)
	r.Index().ExpandWildcardImports()
	r.ResolveDocument("t.sysml", doc)
	wantNames(t, "exposed set of v", exposedNames(t, m, sym(t, root, "v")), []string{"A", "B"})
}

// What a view exposes are memberships of the view (an Expose is an Import), so
// an Expose of the view's own members takes them, each by its own Expose.
func TestExposedElementsOfAViewsExposedMemberships(t *testing.T) {
	m, root := buildModel(t, `
		package Lib { part def A; private part def B; }
		package Mid { private import Lib::*; }
		view inner { expose Mid::*; }
		view outer { expose inner::*; }
	`)
	wantNames(t, "exposed set of outer", exposedNames(t, m, sym(t, root, "outer")), []string{"A"})
}

// An ordinary import nested in its target took the target's public memberships
// only, so an Expose of the nested namespace finds no private one there.
func TestExposedElementsOfAnImportNestedInItsTarget(t *testing.T) {
	const src = `
		package Lib { part def X; private part def P; }
		package Outer {
			private import Lib::*;
			private part def OwnPriv;
			part def OwnPub;
			package Inner { public import Outer::*; }
			package InnerAll { public import all Outer::*; }
		}
		view plain { expose Outer::Inner::*; }
		view all { expose Outer::InnerAll::*; }
	`
	eachIndexRoute(t, func(t *testing.T, expand bool) {
		m, root, r, doc := buildUnresolvedModel(t, "t.sysml", source.KindOf("t.sysml"), src)
		if expand {
			r.Index().ExpandWildcardImports()
		}
		r.ResolveDocument("t.sysml", doc)
		wantNames(t, "exposed set of plain", exposedNames(t, m, sym(t, root, "plain")), []string{"Inner", "InnerAll", "OwnPub"})
		wantNames(t, "exposed set of all", exposedNames(t, m, sym(t, root, "all")),
			[]string{"Inner", "InnerAll", "OwnPriv", "OwnPub", "X"})
	})
}
