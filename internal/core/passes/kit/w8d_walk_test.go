package kit

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

func TestW8DSymbolWalkCachePreservesOrder(t *testing.T) {
	root := symbols.NewScope(nil, nil)
	nested := &symbols.Symbol{Name: "nested"}
	nested.Scope = symbols.NewScope(root, nil)
	nested.Scope.Define("nested", nested)
	root.Define("nested", nested)
	child := symbols.NewScope(root, nil)
	childSymbol := &symbols.Symbol{Name: "child"}
	child.Define("child", childSymbol)
	root.AddChild(child)

	want := collectSymbols(root)
	ctx := NewContext("test.sysml", source.KindSysML, symbols.NewIndex(), nil, Options{}, semantics.NewModel)
	var got []*symbols.Symbol
	WalkSymbols(ctx, root, func(sym *symbols.Symbol) {
		got = append(got, sym)
	})
	if len(got) != len(want) {
		t.Fatalf("cached walk visited %d symbols, direct walk visited %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cached walk symbol %d = %p, direct walk = %p", i, got[i], want[i])
		}
	}
	if cached := Symbols(ctx, root); len(cached) != len(want) {
		t.Fatalf("cached symbol slice has %d symbols, want %d", len(cached), len(want))
	}
}

func TestW8CSymbolWalkCachePreservesOrder(t *testing.T) {
	root := symbols.NewScope(nil, nil)
	nested := &symbols.Symbol{Name: "nested"}
	nested.Scope = symbols.NewScope(root, nil)
	nested.Scope.Define("leaf", &symbols.Symbol{Name: "leaf"})
	root.Define("nested", nested)

	direct := &Walker{}
	var want []*symbols.Symbol
	direct.Walk(root, func(sym *symbols.Symbol) {
		want = append(want, sym)
	})
	ctx := NewContext("test.sysml", source.KindSysML, symbols.NewIndex(), nil, Options{}, semantics.NewModel)
	w := &Walker{Ctx: ctx}
	var got []*symbols.Symbol
	w.Walk(root, func(sym *symbols.Symbol) {
		got = append(got, sym)
	})
	if len(got) != len(want) {
		t.Fatalf("cached W8C walk visited %d symbols, direct walk visited %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cached W8C walk symbol %d = %p, direct walk = %p", i, got[i], want[i])
		}
	}
}

func TestW8CWalkerDeduplicatesOverlappingScopes(t *testing.T) {
	root := symbols.NewScope(nil, nil)
	shared := &symbols.Symbol{Name: "shared"}
	overlap := symbols.NewScope(root, nil)
	overlap.Define("shared", shared)
	root.Define("shared", shared)

	w := &Walker{}
	var got []*symbols.Symbol
	w.Walk(root, func(sym *symbols.Symbol) {
		got = append(got, sym)
	})
	w.Walk(overlap, func(sym *symbols.Symbol) {
		got = append(got, sym)
	})
	if len(got) != 1 || got[0] != shared {
		t.Fatalf("overlapping W8C walk visited %v, want one shared symbol", got)
	}
}
