package query

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Type::isAbstract is read for every feature, a connector's end included: an
// end is a ReferenceUsage, abstract only when declared so, so it reads false.
func TestIsAbstractReadsAConnectorEnd(t *testing.T) {
	file := source.New("ends.sysml", []byte(`package Demo {
		part def D { part a; part b; connection c connect a to b; }
	}`))
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics = %v", p.Diagnostics)
	}
	index := symbols.NewIndex()
	index.AddDocument(file.Name(), root)
	connectors := index.LookupQualified("Demo::D::c")
	if len(connectors) == 0 || connectors[0].Scope == nil {
		t.Fatal("index has no Demo::D::c connection")
	}
	reader := NewPropertyReader(index, nil, nil)
	ends := 0
	connectors[0].Scope.ForEachMember(func(end *symbols.Symbol) bool {
		if end.Kind != symbols.SymbolConnectorEnd {
			return true
		}
		ends++
		values, ok := reader.Values(end, PropertyIsAbstract)
		if !ok || len(values) != 1 || values[0] != "false" {
			t.Errorf("end %d: isAbstract = %v, %v; want [false], true", ends, values, ok)
		}
		return true
	})
	if ends != 2 {
		t.Errorf("%d connector ends, want 2", ends)
	}
}
