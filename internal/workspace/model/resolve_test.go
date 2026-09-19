package model

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestResolveQualifiedInDocResolvesPackage(t *testing.T) {
	ws := NewWorkspace()
	name := "w.sysml"
	ws.Open(name, []byte("package P { namespace N; }"), 1)
	doc := ws.Document(name)
	if doc == nil || doc.Scope == nil {
		t.Fatal("document/scope missing")
	}
	qn := &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "P"}}}
	sym, ok := ws.ResolveQualifiedInDoc(name, doc.Scope, qn)
	if !ok || sym == nil || sym.Name != "P" {
		t.Fatalf("ResolveQualifiedInDoc = %v, %v; want symbol P", sym, ok)
	}
}
