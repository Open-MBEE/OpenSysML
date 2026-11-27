package identity

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// owned declares a symbol of that name in the scope owner owns.
func owned(name string, in *symbols.Scope) *symbols.Symbol {
	sym := &symbols.Symbol{Name: name, OwnerScope: in}
	in.Define(name, sym)
	return sym
}

// Symbols that own each other have no id; asking does not recurse without end.
func TestQualifiedEndsOnAnOwnershipCycle(t *testing.T) {
	inA, inB := symbols.NewScope(nil, nil), symbols.NewScope(nil, nil)
	a, b := owned("a", inB), owned("b", inA)
	inA.SetOwner(a)
	inB.SetOwner(b)
	inSelf := symbols.NewScope(nil, nil)
	self := owned("self", inSelf)
	inSelf.SetOwner(self)

	q := &qualifier{memo: map[*symbols.Symbol]bool{}}
	for _, sym := range []*symbols.Symbol{a, b, self, nil} {
		if q.qualified(sym) {
			t.Errorf("%v on an ownership cycle is qualified", sym)
		}
	}
}

// The guard leaves an ordinary owner chain qualified.
func TestQualifiedFollowsAnOwnerChain(t *testing.T) {
	top := symbols.NewScope(nil, nil)
	pkg := owned("P", top)
	inPkg := symbols.NewScope(top, nil)
	inPkg.SetOwner(pkg)
	part := owned("part", inPkg)

	q := &qualifier{memo: map[*symbols.Symbol]bool{}}
	if !q.qualified(part) || !q.qualified(pkg) {
		t.Error("a named member of a named package is not qualified")
	}
}
