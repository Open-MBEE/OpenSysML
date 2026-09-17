package suggest

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// A table refreshed with what the index changed equals one swept afresh, across
// renames, a removed document, and a wildcard re-export gained and lost.
func TestRefreshMatchesAFreshSweep(t *testing.T) {
	docs := map[string]string{
		"lib.sysml": "package Lib { part def Widget; part def <kg> Kilogram; part def 'SA-506'; }",
		"a.sysml":   "package A { part def Rocket; part def Stage :> Rocket; }",
		"b.sysml":   "package B { public import Lib::*; part def Rocket; }",
		"c.sysml":   "package C { part def Fin; }",
	}
	idx := indexOf(t, docs)
	idx.TrackChanges()
	table := NewTable(idx)

	steps := []struct {
		name string
		edit func()
	}{
		{"rename a declaration", func() {
			docs["a.sysml"] = "package A { part def Rocket; part def Booster :> Rocket; }"
			addDoc(t, idx, "a.sysml", docs["a.sysml"])
		}},
		{"drop a wildcard import", func() {
			docs["b.sysml"] = "package B { part def Rocket; }"
			addDoc(t, idx, "b.sysml", docs["b.sysml"])
		}},
		{"regain it under a new name", func() {
			docs["b.sysml"] = "package B { public import Lib::*; part def Capsule; }"
			addDoc(t, idx, "b.sysml", docs["b.sysml"])
		}},
		{"remove a document", func() {
			delete(docs, "c.sysml")
			idx.RemoveDocument("c.sysml")
		}},
		{"declare a quoted name", func() {
			docs["c.sysml"] = "package C { part def 'left::right'; }"
			addDoc(t, idx, "c.sysml", docs["c.sysml"])
		}},
	}
	for _, step := range steps {
		step.edit()
		idx.ExpandWildcardImports()
		table.Refresh(idx.TakeChanges().Names)
		if got, want := tableState(table), tableState(NewTable(idx)); got != want {
			t.Fatalf("after %s the refreshed table differs from a fresh sweep:\n%s", step.name, diffLines(want, got))
		}
	}
	if got, want := table.Qualified("Kilogram"), []string{"Lib::Kilogram"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Qualified(Kilogram) = %v, want %v", got, want)
	}
	if got := table.byName["Kilogram"]; len(got) != 2 {
		t.Errorf("Kilogram is filed under %v, want its declaration and B's re-export", got)
	}
	if got := table.Declared("Fin"); len(got) != 0 {
		t.Errorf("Declared(Fin) = %v, want none: C was replaced", got)
	}
	if got, want := table.Unquoted("left"), []string{"left::right"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Unquoted(left) = %v, want %v", got, want)
	}
}

// A table refreshed with a name the index never registered stays as it was.
func TestRefreshIgnoresUnregisteredNames(t *testing.T) {
	idx := indexOf(t, map[string]string{"a.sysml": "package A { part def Rocket; }"})
	table := NewTable(idx)
	want := tableState(table)
	table.Refresh(map[string]bool{"\x00judgment/a.sysml": true, "Nowhere::Nothing": true})
	if got := tableState(table); got != want {
		t.Errorf("refreshing unregistered names changed the table:\n%s", diffLines(want, got))
	}
	NewTable(nil).Refresh(map[string]bool{"A::Rocket": true})
}

// tableState renders everything a table answers from: the simple names in
// order, the spellings under each, and the names grouped by length.
func tableState(t *Table) string {
	var b strings.Builder
	fmt.Fprintf(&b, "sorted %v\n", t.sorted)
	for _, name := range t.sorted {
		fqns := append([]string(nil), t.byName[name]...)
		sort.Strings(fqns)
		fmt.Fprintf(&b, "%s -> %v\n", name, fqns)
	}
	lengths := make([]int, 0, len(t.byLength))
	for n := range t.byLength {
		lengths = append(lengths, n)
	}
	sort.Ints(lengths)
	for _, n := range lengths {
		names := make([]string, 0, len(t.byLength[n]))
		for _, c := range t.byLength[n] {
			names = append(names, c.name+"/"+c.lower)
		}
		sort.Strings(names)
		fmt.Fprintf(&b, "len %d -> %v\n", n, names)
	}
	filed := make([]string, 0, len(t.simple))
	for fqn, last := range t.simple {
		filed = append(filed, fqn+"="+last)
	}
	sort.Strings(filed)
	fmt.Fprintf(&b, "filed %v\n", filed)
	return b.String()
}

func diffLines(want, got string) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")
	seen := map[string]bool{}
	for _, l := range wantLines {
		seen[l] = true
	}
	var b strings.Builder
	for _, l := range gotLines {
		if !seen[l] {
			fmt.Fprintf(&b, "+ %s\n", l)
		}
	}
	seen = map[string]bool{}
	for _, l := range gotLines {
		seen[l] = true
	}
	for _, l := range wantLines {
		if !seen[l] {
			fmt.Fprintf(&b, "- %s\n", l)
		}
	}
	return b.String()
}

func indexOf(t *testing.T, docs map[string]string) *symbols.Index {
	t.Helper()
	idx := symbols.NewIndex()
	names := make([]string, 0, len(docs))
	for name := range docs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		addDoc(t, idx, name, docs[name])
	}
	idx.ExpandWildcardImports()
	return idx
}

func addDoc(t *testing.T, idx *symbols.Index, name, src string) {
	t.Helper()
	idx.AddDocument(name, parsedRoot(t, name, src))
}

func parsedRoot(t *testing.T, name, src string) *ast.RootNamespace {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("%s: parse diagnostics: %v", name, p.Diagnostics)
	}
	return root
}
