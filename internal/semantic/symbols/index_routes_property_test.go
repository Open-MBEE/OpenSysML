package symbols

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Route bookkeeping is what decides whether a filtered wildcard import lets a
// name through, and it is derived incrementally over rounds, so a hand-written
// case only covers the graph it draws. These properties hold for every import
// graph, cycles included:
//
//   - expansion settles;
//   - a namespace re-exports a name exactly along the import paths that reach
//     it, each conditioned by the filters on that path;
//   - the routes kept are the ones no other admits more than — a longer
//     condition list admits less, so keeping it would only cost work.
//
// The reference is the direct one: enumerate the paths.

// importEdge is one generated wildcard import: importer imports target, with a
// filter clause of its own or not.
type importEdge struct {
	importer, target int
	private          bool
	filtered         bool
}

func TestReexportRoutesMatchTheImportPathsThatReachAName(t *testing.T) {
	rng := rand.New(rand.NewSource(20260814))
	for trial := 0; trial < 300; trial++ {
		pkgs := 3 + rng.Intn(3)
		var edges []importEdge
		for importer := 0; importer < pkgs; importer++ {
			for k := rng.Intn(3); k > 0; k-- {
				target := rng.Intn(pkgs)
				if target == importer {
					continue
				}
				edges = append(edges, importEdge{
					importer: importer,
					target:   target,
					private:  rng.Intn(4) == 0,
					filtered: rng.Intn(2) == 0,
				})
			}
		}

		src := importGraphSource(pkgs, edges)
		idx := NewIndex()
		addDoc(t, idx, "graph.sysml", src)

		done := make(chan struct{})
		go func() { defer close(done); idx.ExpandWildcardImports() }()
		select {
		case <-done:
		case <-time.After(30 * time.Second):
			t.Fatalf("trial %d: expansion did not settle on\n%s", trial, src)
		}

		assertRoutesAreAnAntichain(t, idx, fmt.Sprintf("trial %d", trial), src)

		x := lookupOne(t, idx, "P0::X")
		want := publicRoutesByPackage(pkgs, edges)
		for pkg := 1; pkg < pkgs; pkg++ {
			fqn := fmt.Sprintf("P%d::X", pkg)
			got := routeLabels(idx.ReexportGates("", fqn, x, ""))
			if !sameRouteSets(got, want[pkg]) {
				t.Fatalf("trial %d: %s is gated by %v, want %v, on\n%s", trial, fqn, got, want[pkg], src)
			}
		}
	}
}

// Routes are derived incrementally, purging and re-deriving the importers an
// edit can have reached, so an index edited into a state must hold the routes a
// fresh build of that state holds — the same contract as the index cache.
func TestReexportRoutesSurviveEditingDocuments(t *testing.T) {
	rng := rand.New(rand.NewSource(20260815))
	for trial := 0; trial < 200; trial++ {
		pkgs := 3 + rng.Intn(3)
		var edges []importEdge
		for importer := 0; importer < pkgs; importer++ {
			for k := rng.Intn(3); k > 0; k-- {
				if target := rng.Intn(pkgs); target != importer {
					edges = append(edges, importEdge{
						importer: importer,
						target:   target,
						private:  rng.Intn(4) == 0,
						filtered: rng.Intn(2) == 0,
					})
				}
			}
		}
		docs := importGraphDocuments(pkgs, edges)

		idx := NewIndex()
		for name, src := range docs {
			addDoc(t, idx, name, src)
		}
		idx.ExpandWildcardImports()

		// Edit: drop a document, and put some back, expanding after each.
		live := make(map[string]string, len(docs))
		for name, src := range docs {
			live[name] = src
		}
		names := sortedNames(docs)
		var edits []string
		for op := rng.Intn(5) + 1; op > 0; op-- {
			name := names[rng.Intn(len(names))]
			if _, held := live[name]; held {
				delete(live, name)
				idx.RemoveDocument(name)
				edits = append(edits, "-"+name)
			} else {
				live[name] = docs[name]
				addDoc(t, idx, name, docs[name])
				edits = append(edits, "+"+name)
			}
			idx.ExpandWildcardImports()
			// Compare after every edit, not only at the end: an edit whose effect a
			// later one hides would otherwise pass.
			stage := fmt.Sprintf("trial %d after %v", trial, edits)
			assertRoutesAreAnAntichain(t, idx, stage, strings.Join(sortedSources(live), ""))
			assertMatchesAFreshBuild(t, idx, live, pkgs, stage)
		}
	}
}

// assertRoutesAreAnAntichain fails if a claim holds two routes of the same
// visibility where one's conditions are a subset of the other's: the wider one
// already admits everything the narrower does, so keeping both only costs work
// and, on a cycle, never settles.
func assertRoutesAreAnAntichain(t *testing.T, idx *Index, stage, src string) {
	t.Helper()
	for _, key := range idx.reexportDocs.keys() {
		for doc, claim := range idx.reexportDocs.at(key) {
			for i, route := range claim.routes {
				for j, other := range claim.routes {
					if i == j || route.private != other.private {
						continue
					}
					if filtersSubsume(other.filters, route.filters) {
						t.Fatalf("%s: %s in %s is reached by both %v and the route %v it subsumes, on\n%s",
							stage, key.fqn, doc, routeLabels([][]ElementFilter{other.filters}),
							routeLabels([][]ElementFilter{route.filters}), src)
					}
				}
			}
		}
	}
}

// assertMatchesAFreshBuild fails unless an edited index holds the names, and the
// routes to them, that an index built from the same documents holds.
func assertMatchesAFreshBuild(t *testing.T, idx *Index, live map[string]string, pkgs int, stage string) {
	t.Helper()
	fresh := NewIndex()
	for _, name := range sortedNames(live) {
		addDoc(t, fresh, name, live[name])
	}
	fresh.ExpandWildcardImports()

	if got, want := idx.FQNs(), fresh.FQNs(); !sameStrings(got, want) {
		t.Fatalf("%s the index holds %v, want %v as freshly built, on\n%s",
			stage, got, want, strings.Join(sortedSources(live), ""))
	}
	if _, declared := live[memberDoc(0)]; !declared {
		return // nothing declares X any more
	}
	edited, built := lookupOne(t, idx, "P0::X"), lookupOne(t, fresh, "P0::X")
	for pkg := 1; pkg < pkgs; pkg++ {
		fqn := fmt.Sprintf("P%d::X", pkg)
		for _, from := range []string{"", fmt.Sprintf("P%d", pkg)} {
			got := routeLabels(idx.ReexportGates("", fqn, edited, from))
			want := routeLabels(fresh.ReexportGates("", fqn, built, from))
			if !sameRouteSets(got, want) {
				t.Fatalf("%s, %s from %q is gated by %v, want %v as freshly built, on\n%s",
					stage, fqn, from, got, want, strings.Join(sortedSources(live), ""))
			}
		}
	}
}

// importGraphDocuments writes the generated graph two documents per package —
// its imports, and the member it declares — so an edit can take either a
// namespace's imports or the members its importers re-export away, and put them
// back.
func importGraphDocuments(pkgs int, edges []importEdge) map[string]string {
	out := make(map[string]string, 2*pkgs)
	for pkg := 0; pkg < pkgs; pkg++ {
		out[fmt.Sprintf("p%d.sysml", pkg)] = importsSource(pkg, edges, false)
		out[memberDoc(pkg)] = memberSource(pkg)
	}
	return out
}

// memberDoc names the document declaring the member of package pkg.
func memberDoc(pkg int) string { return fmt.Sprintf("p%d-member.sysml", pkg) }

// memberSource declares package pkg's own member: X for P0, and one of its own
// elsewhere.
func memberSource(pkg int) string {
	if pkg == 0 {
		return "package P0 {\n\tpart def X;\n}\n"
	}
	return fmt.Sprintf("package P%d {\n\tpart def M%d;\n}\n", pkg, pkg)
}

// sameStrings reports whether two lists hold the same names, whatever order
// they are in.
func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	left, right := append([]string{}, a...), append([]string{}, b...)
	sort.Strings(left)
	sort.Strings(right)
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

// sortedNames returns a map's keys in name order.
func sortedNames(docs map[string]string) []string {
	out := make([]string, 0, len(docs))
	for name := range docs {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// sortedSources returns the documents' sources in name order, for a failure to
// report the model it found.
func sortedSources(docs map[string]string) []string {
	out := make([]string, 0, len(docs))
	for _, name := range sortedNames(docs) {
		out = append(out, docs[name])
	}
	return out
}

// importGraphSource writes the generated graph as one document, giving each
// import's filter clause a condition text of its own so a route can be read back
// as the set of imports that gated it.
func importGraphSource(pkgs int, edges []importEdge) string {
	var b strings.Builder
	for pkg := 0; pkg < pkgs; pkg++ {
		b.WriteString(importsSource(pkg, edges, true))
	}
	return b.String()
}

// importsSource writes one package of the generated graph: the imports it
// states, and, when it declares them here, P0's element X.
func importsSource(pkg int, edges []importEdge, declare bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "package P%d {\n", pkg)
	if declare && pkg == 0 {
		b.WriteString("\tpart def X;\n")
	}
	for i, e := range edges {
		if e.importer != pkg {
			continue
		}
		visibility := "public"
		if e.private {
			visibility = "private"
		}
		fmt.Fprintf(&b, "\t%s import P%d::*", visibility, e.target)
		if e.filtered {
			fmt.Fprintf(&b, "[%s]", edgeLabel(i))
		}
		b.WriteString(";\n")
	}
	b.WriteString("}\n")
	return b.String()
}

// edgeLabel is the condition text the generated filter of edge i carries, which
// identifies that edge in a route.
func edgeLabel(i int) string { return fmt.Sprintf("@F%d", i) }

// publicRoutesByPackage returns, per package, the conditions of the routes its
// public re-export of P0::X is reached by: one per import path from P0, carrying
// the filters that path crosses, and only the ones no other is a subset of. A
// path only continues past a namespace whose import was public, since a private
// import is not re-exported onward.
func publicRoutesByPackage(pkgs int, edges []importEdge) map[int][]map[string]bool {
	out := make(map[int][]map[string]bool)
	visited := make([]bool, pkgs)
	visited[0] = true

	var walk func(at int, carried map[string]bool)
	walk = func(at int, carried map[string]bool) {
		for i, e := range edges {
			// A private import neither exports the name from the importing
			// namespace nor carries it onward, so it is no route of its own.
			if e.target != at || visited[e.importer] || e.private {
				continue
			}
			route := make(map[string]bool, len(carried)+1)
			for label := range carried {
				route[label] = true
			}
			if e.filtered {
				route[edgeLabel(i)] = true
			}
			out[e.importer] = append(out[e.importer], route)
			visited[e.importer] = true
			walk(e.importer, route)
			visited[e.importer] = false
		}
	}
	walk(0, map[string]bool{})

	for pkg, routes := range out {
		out[pkg] = minimalRoutes(routes)
	}
	return out
}

// minimalRoutes drops every route another one is a subset of, which admits at
// least as much as it does.
func minimalRoutes(routes []map[string]bool) []map[string]bool {
	var kept []map[string]bool
	for i, route := range routes {
		redundant := false
		for j, other := range routes {
			if i == j || !subsetOf(other, route) {
				continue
			}
			if subsetOf(route, other) && j > i {
				continue // the same route: keep the first of them
			}
			redundant = true
			break
		}
		if !redundant {
			kept = append(kept, route)
		}
	}
	return kept
}

// subsetOf reports whether every condition of a route is one of b's.
func subsetOf(a, b map[string]bool) bool {
	for label := range a {
		if !b[label] {
			return false
		}
	}
	return true
}

// routeLabels reads the routes the index recorded back as the metadata types
// their conditions classify against, which name the imports that gated them.
func routeLabels(routes [][]ElementFilter) []map[string]bool {
	out := make([]map[string]bool, 0, len(routes))
	for _, route := range routes {
		labels := make(map[string]bool, len(route))
		for _, f := range route {
			labels[filterLabel(f)] = true
		}
		out = append(out, labels)
	}
	return out
}

// filterLabel names the generated condition of one filter, `@F3` for `@F3`.
func filterLabel(f ElementFilter) string {
	e, ok := f.Expr.(*ast.OperatorExpr)
	if !ok || e.TypeRef == nil || len(e.TypeRef.Parts) == 0 {
		return fmt.Sprintf("unreadable(%v)", f.Expr)
	}
	return "@" + e.TypeRef.Parts[len(e.TypeRef.Parts)-1].Text
}

// sameRouteSets reports whether two sets of routes hold the same conditions,
// which also asserts that neither holds a route twice.
func sameRouteSets(a, b []map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	left, right := routeKeys(a), routeKeys(b)
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

// routeKeys renders each route as its sorted conditions, in a fixed order.
func routeKeys(routes []map[string]bool) []string {
	out := make([]string, 0, len(routes))
	for _, route := range routes {
		labels := make([]string, 0, len(route))
		for label := range route {
			labels = append(labels, label)
		}
		sort.Strings(labels)
		out = append(out, strings.Join(labels, "&"))
	}
	sort.Strings(out)
	return out
}
