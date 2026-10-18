package resolve

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// closureModel is specializationModel with the memoized transitive closure the
// live semantic model keeps beside its direct edges.
type closureModel struct {
	specializationModel
	closure map[*symbols.Symbol][]*symbols.Symbol
}

func (m *closureModel) AllSupertypes(sym *symbols.Symbol) []*symbols.Symbol {
	if chain, ok := m.closure[sym]; ok {
		return chain
	}
	seen := map[*symbols.Symbol]bool{sym: true}
	queue := m.DirectSupertypes(sym)
	var chain []*symbols.Symbol
	for len(queue) > 0 {
		sup := queue[0]
		queue = queue[1:]
		if seen[sup] {
			continue
		}
		seen[sup] = true
		chain = append(chain, sup)
		queue = append(queue, m.DirectSupertypes(sup)...)
	}
	m.closure[sym] = chain
	return chain
}

// The resolver takes the specialization chain from the model's closure when the
// model keeps one, in the walk's own order, so a lookup that consults inherited
// imports allocates nothing for the chain; only a model without a closure is walked.
func TestSpecializationChainTakesTheModelsClosure(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": protectedLib,
		"app.sysml": `package App {
			part def Base { protected import Lib::*; }
			part def Mid :> Base;
			part def Leaf :> Mid;
		}`,
	})
	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")
	leaf := scopeOf(t, app, "Leaf").Owner()
	if leaf == nil {
		t.Fatal("Leaf has no owning symbol")
	}

	walked := resolverWithSpecializations(idx)
	want := walked.specializationChain(leaf)
	if len(want) != 2 || want[0].Name != "Mid" || want[1].Name != "Base" {
		t.Fatalf("walked chain of Leaf = %v, want [Mid Base]", names(want))
	}

	r := New(idx)
	r.SetModel(&closureModel{specializationModel{r: r}, map[*symbols.Symbol][]*symbols.Symbol{}})
	if _, ok := r.ResolveName(scopeOf(t, app, "Leaf"), "Pub", ident("Pub")); !ok {
		t.Fatalf("Pub is not visible in Leaf through the closure; diags=%v", r.Diagnostics)
	}
	got := r.specializationChain(leaf)
	if len(got) != len(want) {
		t.Fatalf("closure chain of Leaf = %v, want %v", names(got), names(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("closure chain of Leaf = %v, want %v", names(got), names(want))
		}
	}

	if allocs := testing.AllocsPerRun(100, func() { r.specializationChain(leaf) }); allocs != 0 {
		t.Fatalf("specializationChain allocates %.0f objects per call with a closure, want 0", allocs)
	}
	if allocs := testing.AllocsPerRun(100, func() { walked.specializationChain(leaf) }); allocs == 0 {
		t.Fatal("the walk allocates nothing: the closure no longer saves anything to test")
	}
}

func names(syms []*symbols.Symbol) []string {
	out := make([]string, 0, len(syms))
	for _, sym := range syms {
		out = append(out, sym.Name)
	}
	return out
}
