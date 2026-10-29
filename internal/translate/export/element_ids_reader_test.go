package export

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

const readerModel = "package P {\n\tpart a;\n\tpart a;\n\tpart : X;\n\tpart def X;\n}\n"

func readerIndex(t *testing.T, name string, text string) (*source.SourceFile, *symbols.Index, ElementIDDocument) {
	t.Helper()
	sf := source.New(name, []byte(text))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse %s: %v", name, p.Diagnostics)
	}
	index := libs.NewModelIndex()
	index.AddDocument(name, root)
	index.ExpandWildcardImports()
	resolve.New(index)
	return sf, index, ElementIDDocument{File: sf, Root: root, ParseDiags: p.Diagnostics}
}

// TestElementIDReaderResolvesDeclarationsOfOneDocument: a single document is
// converted over the tree it was read from, so two declarations sharing a
// qualified name are told apart by their nodes, as the conversion wrote them.
func TestElementIDReaderResolvesDeclarationsOfOneDocument(t *testing.T) {
	_, index, doc := readerIndex(t, "dup.sysml", readerModel)
	reader := NewElementIDReader([]ElementIDDocument{doc})
	matches := index.LookupQualified("P::a")
	if len(matches) != 2 {
		t.Fatalf("P::a names %d declarations, want 2", len(matches))
	}
	var ids []string
	for _, sym := range matches {
		id, err := reader.Read(sym.Decl, "P::a")
		if err != nil {
			t.Fatalf("Read P::a: %v", err)
		}
		ids = append(ids, id)
	}
	if ids[0] == "" || ids[1] == "" || ids[0] == ids[1] {
		t.Fatalf("the two declarations of P::a read ids %q, want two distinct ids", ids)
	}
	if id, _ := reader.Read(nil, "P::X"); id != "P__X" {
		t.Fatalf("P::X by name reads %q, want P__X", id)
	}
}

// TestElementIDReaderParsesADocumentHeldWithoutItsTree: a document given by
// its text alone (a workspace's interface record) is parsed for the
// conversion and joins the other documents in it, so its ids are answered.
func TestElementIDReaderParsesADocumentHeldWithoutItsTree(t *testing.T) {
	sf := source.New("parts.sysml", []byte("package Parts { part def Wheel; part wheel : Wheel; }\n"))
	_, _, live := readerIndex(t, "live.sysml", "package Live { private import Parts::*; part car { part w : Wheel; } }\n")
	reader := NewElementIDReader([]ElementIDDocument{{File: sf}, live})
	for fqn, want := range map[string]string{"Parts::wheel": "Parts__wheel", "Live::car": "Live__car"} {
		id, err := reader.Read(nil, fqn)
		if err != nil {
			t.Fatalf("Read %s: %v", fqn, err)
		}
		if id != want {
			t.Fatalf("%s reads %q, want %q", fqn, id, want)
		}
	}
	broken := NewElementIDReader([]ElementIDDocument{{File: source.New("broken.sysml", []byte("package {"))}, live})
	if _, err := broken.Read(nil, "Live::car"); err == nil {
		t.Fatalf("a document the parser cannot read whole does not refuse the model")
	}
}
