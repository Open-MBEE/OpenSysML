package semantics

import "testing"

// Whether a feature is a variation point is asked of every feature on every
// instantiation, so its declared closure is walked once per symbol.
func TestDeclaredClosureIsWalkedOncePerSymbol(t *testing.T) {
	m, root := buildModel(t, variationSource)
	cut := member(t, m, sym(t, root, "family"), "cut")
	chosenCut := member(t, m, sym(t, root, "chosen"), "cut")
	plainCut := member(t, m, sym(t, root, "Diamond"), "cut")
	if !m.IsVariationFeature(cut) || !m.IsVariationFeature(chosenCut) || m.IsVariationFeature(plainCut) {
		t.Fatalf("IsVariationFeature: cut %v, chosen cut %v, plain cut %v, want true, true, false",
			m.IsVariationFeature(cut), m.IsVariationFeature(chosenCut), m.IsVariationFeature(plainCut))
	}
	first := m.AllDeclaredSupertypes(chosenCut)
	if len(first) == 0 {
		t.Fatal("AllDeclaredSupertypes(chosen cut) is empty, want the cut it redefines")
	}
	for name, query := range map[string]func(){
		"AllDeclaredSupertypes":       func() { m.AllDeclaredSupertypes(chosenCut) },
		"IsVariationFeature":          func() { m.IsVariationFeature(chosenCut) },
		"IsVariationFeature of plain": func() { m.IsVariationFeature(plainCut) },
	} {
		if got := testing.AllocsPerRun(20, query); got != 0 {
			t.Errorf("a repeated %s query allocates %.0f times, want 0", name, got)
		}
	}
	if again := m.AllDeclaredSupertypes(chosenCut); len(again) != len(first) || again[0] != first[0] {
		t.Fatalf("memoized AllDeclaredSupertypes(chosen cut) = %v, want %v", again, first)
	}
}
