package resolve

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestPrivateImportVisibilityKeepsNamespaceBoundaries(t *testing.T) {
	r := New(symbols.NewIndex())
	for _, test := range []struct {
		target, from string
		want         bool
	}{
		{"", "", false}, {"", "Child", false}, {"Target", "", false},
		{"Target", "Target", true}, {"Target", "Target::Child", true},
		{"Target", "TargetSuffix", false}, {"Target", "Target:Child", false},
		{"Target", "Target::", true},
		{"'Quoted Namespace'", "'Quoted Namespace'::Child", true},
		{"Δοκιμή", "Δοκιμή::Child", true}, {"Δοκιμή", "ΔοκιμήSuffix", false},
	} {
		target := &symbols.Symbol{Name: test.target}
		from := symbols.NewScope(nil, nil)
		from.SetOwner(&symbols.Symbol{Name: test.from})
		for _, imp := range []*ast.Import{{Visibility: ast.VisibilityPrivate}, {IsExpose: true}} {
			if got := r.importVisibleFrom(target, from, imp); got != test.want {
				t.Errorf("%q from %q: %v, want %v", test.target, test.from, got, test.want)
			}
		}
	}
}

func TestPrivateImportVisibilityDoesNotAllocateNamespacePrefix(t *testing.T) {
	r := New(symbols.NewIndex())
	target := &symbols.Symbol{Name: "LongNamespaceNameThatExceedsTheStackStringBuffer"}
	from := symbols.NewScope(nil, nil)
	from.SetOwner(&symbols.Symbol{Name: target.Name + "::Child"})
	imp := &ast.Import{Visibility: ast.VisibilityPrivate}
	if allocations := testing.AllocsPerRun(100, func() {
		if !r.importVisibleFrom(target, from, imp) {
			t.Fatal("child rejected")
		}
	}); allocations != 0 {
		t.Fatalf("namespace containment allocated %.0f objects", allocations)
	}
}

func BenchmarkPrivateImportVisibility(b *testing.B) {
	r := New(symbols.NewIndex())
	target := &symbols.Symbol{Name: "LongNamespaceNameThatExceedsTheStackStringBuffer"}
	from := symbols.NewScope(nil, nil)
	from.SetOwner(&symbols.Symbol{Name: target.Name + "::Child"})
	imp := &ast.Import{Visibility: ast.VisibilityPrivate}
	b.ReportAllocs()
	for b.Loop() {
		if !r.importVisibleFrom(target, from, imp) {
			b.Fatal("child rejected")
		}
	}
}
