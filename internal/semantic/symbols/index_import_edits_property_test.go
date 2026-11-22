package symbols

import (
	"fmt"
	"maps"
	"math/rand"
	"slices"
	"strings"
	"testing"
	"time"
)

// An index edited into a state holds the import cycles, routes and hops a fresh
// build of it holds, and settles within the route bound whatever edit led there.
func TestImportRoutesSurviveEditingImports(t *testing.T) {
	rng := rand.New(rand.NewSource(20261121))
	compared, otherNames := 0, 0
	for trial := 0; trial < 1000; trial++ {
		pkgs := 3 + rng.Intn(3)
		live := make(map[string]string, pkgs)
		idx := NewIndex()
		for pkg := 0; pkg < pkgs; pkg++ {
			name := fmt.Sprintf("p%d.sysml", pkg)
			live[name] = randomImportingPackage(rng, pkg, pkgs)
			addDoc(t, idx, name, live[name])
		}
		expandOrFail(t, idx, fmt.Sprintf("trial %d", trial), live)
		if !assertImportStateOfAFreshBuild(t, idx, live, fmt.Sprintf("trial %d", trial)) {
			t.Fatalf("trial %d: a fresh build differs from a fresh build", trial)
		}

		// Imports name nested namespaces bare, so an edit re-routes other imports.
		var edits []string
		for op := 1 + rng.Intn(5); op > 0; op-- {
			pkg := rng.Intn(pkgs)
			name := fmt.Sprintf("p%d.sysml", pkg)
			if _, held := live[name]; held && rng.Intn(4) == 0 {
				delete(live, name)
				idx.RemoveDocument(name)
				edits = append(edits, "-"+name)
			} else {
				live[name] = randomImportingPackage(rng, pkg, pkgs)
				addDoc(t, idx, name, live[name])
				edits = append(edits, "~"+name)
			}
			stage := fmt.Sprintf("trial %d after %v", trial, edits)
			expandOrFail(t, idx, stage, live)
			if !assertImportStateOfAFreshBuild(t, idx, live, stage) {
				// Which names an edited index holds is not what this compares.
				otherNames++
				break
			}
			compared++
		}
	}
	if compared < 10*otherNames {
		t.Errorf("%d edited states compared, %d holding other names than a fresh build: too few to tell", compared, otherNames)
	}
}

// randomImportingPackage writes package pkg: a member, a private one, sometimes
// a nested namespace with a member and imports of its own, and its imports.
func randomImportingPackage(rng *rand.Rand, pkg, pkgs int) string {
	imports := func(indent string) string {
		var b strings.Builder
		for k := rng.Intn(3); k > 0; k-- {
			to := rng.Intn(pkgs)
			target := fmt.Sprintf("P%d", to)
			switch rng.Intn(4) {
			case 0:
				target = fmt.Sprintf("N%d", to)
			case 1:
				target += fmt.Sprintf("::N%d", to)
			}
			visibility, all := "public", ""
			if rng.Intn(3) == 0 {
				visibility = "private"
			}
			if rng.Intn(3) == 0 {
				all = "all "
			}
			fmt.Fprintf(&b, "%s%s import %s%s::*;\n", indent, visibility, all, target)
		}
		return b.String()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "package P%d {\n\tpart def M%d;\n\tprivate part def H%d;\n", pkg, pkg, pkg)
	if rng.Intn(2) == 0 {
		fmt.Fprintf(&b, "\tpackage N%d {\n\t\tpart def K%d;\n%s\t}\n", pkg, pkg, imports("\t\t"))
	}
	b.WriteString(imports("\t"))
	b.WriteString("}\n")
	return b.String()
}

// expandOrFail expands the wildcard imports, failing if that does not settle.
func expandOrFail(t *testing.T, idx *Index, stage string, live map[string]string) {
	t.Helper()
	done := make(chan struct{})
	go func() { defer close(done); idx.ExpandWildcardImports() }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatalf("%s: expansion did not settle on\n%s", stage, strings.Join(sortedSources(live), ""))
	}
	// A claim keeps the routes no other subsumes: without filters, one per
	// visibility and set of namespaces it came through privately.
	for _, key := range idx.reexportDocs.keys() {
		for doc, claim := range idx.reexportDocs.at(key) {
			if len(claim.routes) > maxRoutesOfAClaim {
				t.Fatalf("%s: %s in %s holds %d routes, want at most %d, on\n%s",
					stage, key.fqn, doc, len(claim.routes), maxRoutesOfAClaim, strings.Join(sortedSources(live), ""))
			}
		}
	}
}

// maxRoutesOfAClaim bounds the routes one document's claim on a name keeps over
// the generated graphs, of at most ten namespaces with three imports each.
const maxRoutesOfAClaim = 8

// assertImportStateOfAFreshBuild fails unless idx holds the import state of an
// index built from live, comparing nothing (false) when their names differ.
func assertImportStateOfAFreshBuild(t *testing.T, idx *Index, live map[string]string, stage string) bool {
	t.Helper()
	fresh := NewIndex()
	for _, name := range sortedNames(live) {
		addDoc(t, fresh, name, live[name])
	}
	fresh.ExpandWildcardImports()
	if !slices.Equal(idx.FQNs(), fresh.FQNs()) {
		return false
	}
	if got, want := importState(idx), importState(fresh); got != want {
		t.Fatalf("%s the index differs from a fresh build:\n%s\non\n%s",
			stage, diffLines(want, got), strings.Join(sortedSources(live), ""))
	}
	return true
}

// importState renders what expansion derived: each namespace's import cycle,
// and each re-export's routes, hops and answers with one namespace excluded.
func importState(idx *Index) string {
	var lines []string
	namespaces := idx.wildcardMeta.keys()
	for _, ns := range idx.cyclic.keys() {
		lines = append(lines, fmt.Sprintf("cycle %s: %v", ns, idx.cyclic.at(ns)))
	}
	for _, key := range idx.reexportDocs.keys() {
		declared, _ := idx.declaredAt.get(key.sym)
		claims := idx.reexportDocs.at(key)
		for _, doc := range slices.Sorted(maps.Keys(claims)) {
			claim := claims[doc]
			var routes, hops []string
			for _, route := range claim.routes {
				routes = append(routes, fmt.Sprintf("private=%t filters=%d via=%v", route.private, len(route.filters), route.via))
			}
			for _, hop := range claim.hops {
				hops = append(hops, fmt.Sprintf("%s private=%t all=%t", hop.from, hop.private, hop.all))
			}
			slices.Sort(routes)
			slices.Sort(hops)
			lines = append(lines, fmt.Sprintf("%s = %s in %s public=%t routes=%v hops=%v",
				key.fqn, declared, doc, claim.public, routes, hops))
		}
		parent, _ := splitFQN(key.fqn)
		for _, excluded := range namespaces {
			for _, from := range []string{"", parent} {
				routed, accepted := idx.EachReexportGate("", key.fqn, key.sym, from, []string{excluded},
					func([]ElementFilter) bool { return true })
				lines = append(lines, fmt.Sprintf("%s = %s from %q excluding %s: routed=%t accepted=%t",
					key.fqn, declared, from, excluded, routed, accepted))
			}
		}
	}
	slices.Sort(lines)
	return strings.Join(lines, "\n")
}
