package symbols

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestOrigin(t *testing.T) {
	var none *Symbol
	if got := none.Origin(); got.Located() {
		t.Fatalf("nil symbol: got %+v, want unlocated", got)
	}
	synthetic := &Symbol{Name: "x"}
	if got := synthetic.Origin(); got.Located() {
		t.Fatalf("synthetic symbol: got %+v, want unlocated", got)
	}
	decl := source.Span{Offset: 4, Len: 10}
	name := source.Span{Offset: 9, Len: 1}
	sym := &Symbol{Name: "x", DocName: "a.sysml", DeclSpan: decl, NameSpan: name}
	got := sym.Origin()
	if !got.Located() || got.Doc != "a.sysml" || got.Span != decl || got.Name != name {
		t.Fatalf("declared symbol: got %+v", got)
	}
	if got := OriginAt("", decl); got.Located() {
		t.Fatalf("no document: got %+v, want unlocated", got)
	}
	if got := NodeOrigin("a.sysml", nil); got.Located() {
		t.Fatalf("nil node: got %+v, want unlocated", got)
	}
}
