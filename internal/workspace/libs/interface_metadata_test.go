package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestInterfaceWriterRecordsPrefixMetadataType(t *testing.T) {
	const name = "metadata.sysml"
	idx := indexOf(t, name, `package P {
		metadata def Marker;
		part run { @Marker; }
	}`)
	r := resolve.New(idx)
	model := semantics.NewModel(r)
	r.SetModel(model)
	root := idx.DocumentRoot(name)
	r.ResolveDocument(name, root.Node().(*ast.RootNamespace))

	var prefix *symbols.Symbol
	var visit func(*symbols.Scope)
	visit = func(scope *symbols.Scope) {
		for _, sym := range scope.AllMembers() {
			if _, ok := sym.Decl.(*ast.PrefixMetadata); ok {
				prefix = sym
				return
			}
			if sym.Scope != nil {
				visit(sym.Scope)
				if prefix != nil {
					return
				}
			}
		}
	}
	visit(root)
	if prefix == nil {
		t.Fatal("prefix metadata symbol was not indexed")
	}

	writer := &interfaceWriter{idx: idx, r: r, model: model}
	facts := writer.facts(prefix)
	if writer.err != nil {
		t.Fatalf("record prefix metadata: %v", writer.err)
	}
	if facts.Node != symbols.NodePrefixMetadata {
		t.Errorf("node = %v, want PrefixMetadata", facts.Node)
	}
	if got := facts.MetadataType.FQN; got != "P::Marker" {
		t.Errorf("MetadataType = %q, want P::Marker", got)
	}
	if facts.MetadataType.Doc != name {
		t.Errorf("MetadataType document = %q, want %q", facts.MetadataType.Doc, name)
	}
	if facts.MetadataType.IsZero() {
		t.Error("MetadataType is empty")
	}
}
