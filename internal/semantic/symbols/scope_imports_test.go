package symbols

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A parsed scope collects its imports once: a namespace of many members is read on every
// lookup through it, so a repeated call must not scan the members again.
func TestScopeImportsCollectedOnce(t *testing.T) {
	first, second := &ast.Import{}, &ast.Import{}
	members := []ast.Node{first, &ast.Membership{Member: second}}
	for i := 0; i < 64; i++ {
		members = append(members, &ast.Usage{})
	}
	s := NewScope(nil, &ast.Package{Members: members})

	got := s.Imports()
	if len(got) != 2 || got[0] != first || got[1] != second {
		t.Fatalf("Imports = %v, want the two imports in declaration order", got)
	}
	if allocs := testing.AllocsPerRun(10, func() { _ = s.Imports() }); allocs != 0 {
		t.Fatalf("a repeated Imports allocated %v times, want 0", allocs)
	}
	if again := s.Imports(); len(again) != 2 || &again[0] != &got[0] {
		t.Fatal("a repeated Imports did not return the collected imports")
	}
	if none := NewScope(nil, &ast.Package{}).Imports(); len(none) != 0 {
		t.Fatalf("Imports of a namespace stating none = %v", none)
	}
}
