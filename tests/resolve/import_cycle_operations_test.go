package resolve_test

import (
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// cycleImport is one wildcard import of an import graph: its target namespace,
// its visibility and whether it is an `import all`.
type cycleImport struct {
	target      int
	public, all bool
}

// cycleGraph is the imports each namespace N<i> owns. N<i> declares a private
// part definition H<i> and a public one V<i>, so every name has one owner.
type cycleGraph [][]cycleImport

// visible is Namespace::visibleMemberships(excluded, false, includeAll) of
// KerML 8.3.2.4.5 over the graph, as the set of names it holds.
func (g cycleGraph) visible(ns int, excluded []int, includeAll bool) map[string]bool {
	out := map[string]bool{fmt.Sprintf("V%d", ns): true}
	if includeAll {
		out[fmt.Sprintf("H%d", ns)] = true
	}
	excluded = append(slices.Clone(excluded), ns)
	for _, imp := range g[ns] {
		// NamespaceImport::importedMemberships (8.3.2.4.6): nothing from an excluded namespace.
		if (!includeAll && !imp.public) || slices.Contains(excluded, imp.target) {
			continue
		}
		for name := range g.visible(imp.target, excluded, imp.all) {
			out[name] = true
		}
	}
	return out
}

func (g cycleGraph) source() string {
	var b strings.Builder
	names := g.names()
	probe := func(pkg, lead string) {
		for _, name := range names {
			fmt.Fprintf(&b, "\tpart %s_%s : %s%s;\n", pkg, name, lead, name)
		}
	}
	for ns, imports := range g {
		fmt.Fprintf(&b, "package N%d {\n\tprivate part def H%d;\n\tpublic part def V%d;\n", ns, ns, ns)
		for _, imp := range imports {
			visibility, all := "private", ""
			if imp.public {
				visibility = "public"
			}
			if imp.all {
				all = " all"
			}
			fmt.Fprintf(&b, "\t%s import%s N%d::*;\n", visibility, all, imp.target)
		}
		probe(fmt.Sprintf("N%d", ns), "")
		b.WriteString("}\n")
		fmt.Fprintf(&b, "package Ordinary%d {\n\tprivate import N%d::*;\n", ns, ns)
		probe(fmt.Sprintf("Ordinary%d", ns), "")
		b.WriteString("}\n")
		fmt.Fprintf(&b, "package All%d {\n\tprivate import all N%d::*;\n", ns, ns)
		probe(fmt.Sprintf("All%d", ns), "")
		b.WriteString("}\n")
		fmt.Fprintf(&b, "package Qualified%d {\n", ns)
		probe(fmt.Sprintf("Qualified%d", ns), fmt.Sprintf("N%d::", ns))
		b.WriteString("}\n")
		fmt.Fprintf(&b, "package Exposing%d { view exposing%d { expose N%d::*; } }\n", ns, ns, ns)
	}
	return b.String()
}

func (g cycleGraph) names() []string {
	var names []string
	for ns := range g {
		names = append(names, fmt.Sprintf("H%d", ns), fmt.Sprintf("V%d", ns))
	}
	return names
}

func (g cycleGraph) String() string {
	var parts []string
	for ns, imports := range g {
		for _, imp := range imports {
			parts = append(parts, fmt.Sprintf("N%d-%s->N%d", ns, map[bool]string{true: "public", false: "private"}[imp.public]+map[bool]string{true: " all", false: ""}[imp.all], imp.target))
		}
	}
	return strings.Join(parts, " ")
}

// check compares what every probe resolves, and what a view exposing each
// namespace takes, with the operations, by the scope route and the expanded index.
func (g cycleGraph) check(t *testing.T) {
	t.Helper()
	const name = "cycle.sysml"
	src := g.source()
	for _, expand := range []bool{false, true} {
		p := parser.New(source.New(name, []byte(src)))
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Fatalf("parse diagnostics: %v", p.Diagnostics)
		}
		idx := symbols.NewIndexFromDoc(name, root)
		if expand {
			idx.ExpandWildcardImports()
		}
		r := resolve.New(idx)
		m := semantics.NewModel(r)
		r.SetModel(m)
		r.ResolveDocument(name, root)
		docRoot := idx.DocumentRoot(name)
		resolved := func(pkg string) []string {
			owner, ok := docRoot.LookupLocal(pkg)
			if !ok {
				t.Fatalf("no package %s", pkg)
			}
			var out []string
			for _, want := range g.names() {
				usage, ok := owner.Scope.LookupLocal(pkg + "_" + want)
				if !ok {
					t.Fatalf("no probe %s_%s", pkg, want)
				}
				for _, sup := range m.DirectSupertypes(usage) {
					out = append(out, sup.Name)
				}
			}
			slices.Sort(out)
			return out
		}
		want := func(set map[string]bool) []string {
			var out []string
			for name := range set {
				out = append(out, name)
			}
			slices.Sort(out)
			return out
		}
		expect := func(what string, got, want []string) {
			if !slices.Equal(got, want) {
				t.Errorf("%s (expanded index %v): %s = %v, the operations give %v", g, expand, what, got, want)
			}
		}
		for ns := range g {
			expect(fmt.Sprintf("names N%d resolves", ns), resolved(fmt.Sprintf("N%d", ns)), want(g.visible(ns, nil, true)))
			expect(fmt.Sprintf("names an import of N%d resolves", ns), resolved(fmt.Sprintf("Ordinary%d", ns)), want(g.visible(ns, nil, false)))
			expect(fmt.Sprintf("names an import all of N%d resolves", ns), resolved(fmt.Sprintf("All%d", ns)), want(g.visible(ns, nil, true)))
			expect(fmt.Sprintf("names qualified by N%d", ns), resolved(fmt.Sprintf("Qualified%d", ns)), want(g.visible(ns, nil, false)))
			exposing, _ := docRoot.LookupLocal(fmt.Sprintf("Exposing%d", ns))
			view, _ := exposing.Scope.LookupLocal(fmt.Sprintf("exposing%d", ns))
			elems, err := m.ExposedElements(view)
			if err != nil {
				t.Fatalf("ExposedElements: %v", err)
			}
			var exposed []string
			for _, elem := range elems {
				if i := strings.LastIndex(elem.Name, "::"); i >= 0 {
					exposed = append(exposed, elem.Name[i+2:])
				} else if !strings.Contains(elem.Name, "_") {
					exposed = append(exposed, elem.Name)
				}
			}
			slices.Sort(exposed)
			expect(fmt.Sprintf("exposed set of N%d", ns), slices.Compact(exposed), want(g.visible(ns, nil, true)))
		}
	}
}

// TestImportCycleFollowsTheOperations checks every cyclic import graph of three
// namespaces, and random ones of four, against KerML 8.3.2.4.5 computed directly.
func TestImportCycleFollowsTheOperations(t *testing.T) {
	kinds := []cycleImport{{}, {all: true}, {public: true}, {public: true, all: true}}
	rng := rand.New(rand.NewSource(1))
	for size, count := range map[int]int{2: 60, 3: 250, 4: 250} {
		for n := 0; n < count; n++ {
			g := make(cycleGraph, size)
			for from := range g {
				for to := range g {
					if from != to && rng.Intn(3) != 0 {
						imp := kinds[rng.Intn(len(kinds))]
						imp.target = to
						g[from] = append(g[from], imp)
					}
				}
			}
			g.check(t)
		}
	}
}
