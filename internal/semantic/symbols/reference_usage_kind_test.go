package symbols

import (
	"testing"
)

// A usage declared without a kind keyword is a ReferenceUsage (SysML v2 §7.6.4):
// `x : Integer;` and `ref x : Integer;` alike, a directed parameter included
// (§7.6.3), while the `attribute` keyword keeps its own kind.
func TestKindlessUsageIsAReferenceUsage(t *testing.T) {
	scope := buildSysML(t, `package P {
		private import ScalarValues::*;
		part def V {
			x : Integer;
			ref r : Integer;
			attribute a : Integer;
			ref attribute ra : Integer;
			action def Act { in energy : Integer; inout io : Integer; out o : Integer; }
		}
	}`)
	pkg, ok := scope.LookupLocal("P")
	if !ok {
		t.Fatal("package P not indexed")
	}
	v, ok := pkg.Scope.LookupLocal("V")
	if !ok {
		t.Fatal("V not indexed")
	}
	want := map[string]SymbolKind{
		"x":  SymbolReferenceUsage,
		"r":  SymbolReferenceUsage,
		"a":  SymbolAttributeUsage,
		"ra": SymbolAttributeUsage,
	}
	for name, kind := range want {
		sym, ok := v.Scope.LookupLocal(name)
		if !ok {
			t.Errorf("%s not indexed", name)
			continue
		}
		if sym.Kind != kind {
			t.Errorf("kind of %s = %v, want %v", name, sym.Kind, kind)
		}
	}
	act, ok := v.Scope.LookupLocal("Act")
	if !ok {
		t.Fatal("Act not indexed")
	}
	for _, name := range []string{"energy", "io", "o"} {
		sym, ok := act.Scope.LookupLocal(name)
		if !ok {
			t.Errorf("parameter %s not indexed", name)
			continue
		}
		if sym.Kind != SymbolReferenceUsage {
			t.Errorf("kind of parameter %s = %v, want referenceUsage", name, sym.Kind)
		}
	}
}
