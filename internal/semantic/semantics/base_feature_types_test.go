package semantics

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func typeNames(types []*symbols.Symbol) string {
	names := make([]string, len(types))
	for i, typ := range types {
		names[i] = typ.Name
	}
	return strings.Join(names, " ")
}

// A base feature's types are derived once: a second query is served the stored
// answer, which holds the types of every feature the base subsets.
func TestBaseFeatureTypesAreMemoized(t *testing.T) {
	m, root := buildModelNamed(t, "t.kerml", `package P {
		classifier A;
		classifier B;
		classifier C specializes A;
		feature general : A;
		feature other : B;
		feature base : C subsets general, other;
	}`)
	base := nestedSym(t, root, "P::base")
	first := m.baseFeatureTypes(base)
	if got := typeNames(first); got != "C B" {
		t.Fatalf("baseFeatureTypes(base) = %q, want %q", got, "C B")
	}
	stored, ok := m.featureBaseTypes[base]
	if !ok || typeNames(stored) != "C B" {
		t.Fatalf("memo holds %q (%v), want the derived answer", typeNames(stored), ok)
	}
	second := m.baseFeatureTypes(base)
	if len(second) != len(first) || &second[0] != &first[0] {
		t.Fatal("the second query derived the answer again")
	}
	if _, ok := m.featureBaseTypes[nestedSym(t, root, "P::general")]; ok {
		t.Fatal("a feature only walked through was memoized without being asked")
	}
}

// Features subsetting each other end the walk; each still supplies its own
// types and those of the other, whichever is asked first.
func TestBaseFeatureTypesOnASubsettingCycle(t *testing.T) {
	const src = `package P {
		classifier A;
		classifier B;
		feature a : A subsets b;
		feature b : B subsets a;
	}`
	for _, order := range [][]string{{"a", "b"}, {"b", "a"}} {
		m, root := buildModelNamed(t, "t.kerml", src)
		want := map[string]string{"a": "A B", "b": "B A"}
		for round := 0; round < 2; round++ {
			for _, name := range order {
				if got := typeNames(m.baseFeatureTypes(nestedSym(t, root, "P::"+name))); got != want[name] {
					t.Errorf("order %v round %d: baseFeatureTypes(%s) = %q, want %q", order, round, name, got, want[name])
				}
			}
		}
	}
}

// The memo is owned by the document declaring the base: replacing it drops the
// entry, and the next query reads the new declaration.
func TestBaseFeatureTypesFollowTheDeclaringDocument(t *testing.T) {
	m, r, replace := trackedModel(t, symbols.NewIndex(), "bases.kerml")
	query := func(scope *symbols.Scope) (*symbols.Symbol, string) {
		base := nestedSym(t, scope, "P::base")
		var types []*symbols.Symbol
		r.InDocument("bases.kerml", func() { types = m.baseFeatureTypes(base) })
		return base, typeNames(types)
	}
	scope := replace(`package P { classifier A; classifier B; feature general : A; feature base subsets general; }`)
	old, got := query(scope)
	if got != "A" {
		t.Fatalf("baseFeatureTypes(base) = %q, want A", got)
	}
	scope = replace(`package P { classifier A; classifier B; feature general : B; feature base subsets general; }`)
	if _, stale := m.featureBaseTypes[old]; stale {
		t.Fatal("the previous document's entry survived the edit")
	}
	if _, got := query(scope); got != "B" {
		t.Fatalf("after the edit baseFeatureTypes(base) = %q, want B", got)
	}
	if len(m.featureBaseTypes) != 1 {
		t.Fatalf("memo holds %d entries, want the current base only", len(m.featureBaseTypes))
	}
}

// An entry also depends on the documents declaring what the base subsets:
// editing one of those drops it, though the base's own document is untouched.
func TestBaseFeatureTypesFollowASubsettedDocument(t *testing.T) {
	idx := symbols.NewIndex()
	m, r, replaceGenerals := trackedModel(t, idx, "generals.kerml")
	replaceGenerals(`package G { classifier A; classifier B; feature general : A; }`)
	bases := parser.New(source.New("bases.kerml", []byte(`package P { feature base subsets G::general; }`))).ParseFile()
	idx.AddDocument("bases.kerml", bases)
	r.Invalidate(idx.TakeChanges())
	r.InDocument("bases.kerml", func() { r.ResolveDocument("bases.kerml", bases) })
	base := nestedSym(t, idx.DocumentRoot("bases.kerml"), "P::base")
	query := func() string {
		var types []*symbols.Symbol
		r.InDocument("bases.kerml", func() { types = m.baseFeatureTypes(base) })
		return typeNames(types)
	}
	if got := query(); got != "A" {
		t.Fatalf("baseFeatureTypes(base) = %q, want A", got)
	}
	replaceGenerals(`package G { classifier A; classifier B; feature general : B; }`)
	if _, stale := m.featureBaseTypes[base]; stale {
		t.Fatal("the entry survived an edit to the document it read")
	}
	if got := query(); got != "B" {
		t.Fatalf("after the edit baseFeatureTypes(base) = %q, want B", got)
	}
}

// declaredReaches follows a declared chain longer than the visited set holds in
// place, and ends on a cycle that does not reach the declaration asked for.
func TestDeclaredReachesOnALongChain(t *testing.T) {
	const length = 80
	var src strings.Builder
	src.WriteString("package P {\n\tclassifier Apart;\n")
	for i := 0; i < length; i++ {
		fmt.Fprintf(&src, "\tclassifier C%d specializes C%d;\n", i, (i+1)%length)
	}
	src.WriteString("\tclassifier Tail specializes C0;\n}\n")
	m, root := buildModelNamed(t, "t.kerml", src.String())
	at := func(name string) *symbols.Symbol { return nestedSym(t, root, "P::"+name) }
	if !m.declaredReaches(at("Tail"), at(fmt.Sprintf("C%d", length-1))) {
		t.Error("the end of the chain is not reached from its start")
	}
	if !m.declaredReaches(at("C5"), at("C4")) {
		t.Error("a declaration on the cycle is not reached around it")
	}
	if m.declaredReaches(at("Tail"), at("Apart")) {
		t.Error("a declaration off the chain is reported reached")
	}
	if m.declaredReaches(at("C0"), at("Tail")) {
		t.Error("the chain is followed against its direction")
	}
	if m.declaredReaches(nil, at("Tail")) || m.declaredReaches(at("Tail"), nil) {
		t.Error("a missing declaration is reported reached")
	}
}

func TestReachSetHoldsEachDeclarationOnce(t *testing.T) {
	var set reachSet
	syms := make([]*symbols.Symbol, 3*len(set.few))
	for i := range syms {
		syms[i] = &symbols.Symbol{}
	}
	for i, s := range syms {
		if !set.add(s) {
			t.Fatalf("declaration %d reported visited on its first visit", i)
		}
	}
	for i, s := range syms {
		if set.add(s) {
			t.Fatalf("declaration %d reported new on its second visit", i)
		}
	}
	if set.n != len(set.few) || len(set.more) != len(syms)-len(set.few) {
		t.Fatalf("set holds %d in place and %d beyond, want %d and %d", set.n, len(set.more), len(set.few), len(syms)-len(set.few))
	}
}
