package symbols

import "testing"

// A succession usage is a SuccessionAsUsage, so it is classified as one rather
// than left unclassified (F52).
func TestW7BSuccessionUsageIsClassified(t *testing.T) {
	scope := buildScope(t, `action def Base {
	action paint;
	action dry;
	succession named first paint then dry;
}`)
	base := scope.LookupLocalAll("Base")[0]
	syms := base.Scope.LookupLocalAll("named")
	if len(syms) != 1 {
		t.Fatalf("expected 1 symbol for `named`, got %d", len(syms))
	}
	if syms[0].Kind != SymbolSuccessionUsage {
		t.Errorf("kind = %v, want %v", syms[0].Kind, SymbolSuccessionUsage)
	}
}

// A succession's kind is distinct from a connection's: they are different
// metaclasses, and a query surface reports each by its own name.
func TestW7BSuccessionKindIsNotConnection(t *testing.T) {
	if SymbolSuccessionUsage == SymbolConnectionUsage {
		t.Fatal("succession and connection share a kind")
	}
	if got := SymbolSuccessionUsage.String(); got != "successionUsage" {
		t.Errorf("String() = %q, want %q", got, "successionUsage")
	}
}
