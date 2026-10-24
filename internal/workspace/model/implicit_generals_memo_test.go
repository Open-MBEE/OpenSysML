package model

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// A feature's implied generals are read on every type query of it, so the model
// derives them once per symbol: a second query answers from the first's slice.
func TestImplicitGeneralsAreDerivedOncePerSymbol(t *testing.T) {
	const uri = "file:///implicit-generals-memo.sysml"
	ws := NewWorkspace()
	ws.Open(uri, []byte("package P { part def Car; part car : Car; }"), 1)
	defer ws.Close(uri)

	pkg, ok := ws.index.DocumentRoot(uri).LookupLocal("P")
	if !ok {
		t.Fatal("package P not found")
	}
	sym, ok := pkg.Scope.LookupLocal("car")
	if !ok {
		t.Fatal("usage car not found")
	}
	r := resolve.New(ws.index)
	m := semantics.NewModel(r)
	r.SetModel(m)

	first := m.ImplicitGenerals(sym)
	if len(first) == 0 {
		t.Fatal("part car implies no generals; want at least Parts::parts")
	}
	second := m.ImplicitGenerals(sym)
	if len(second) != len(first) || &second[0] != &first[0] {
		t.Fatalf("ImplicitGenerals derived the generals twice: %p then %p", &first[0], &second[0])
	}
}
