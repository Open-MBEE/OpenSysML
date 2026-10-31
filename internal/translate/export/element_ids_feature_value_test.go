package export

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// The ids the reader answers for a feature's value expression and for the
// FeatureValue that owns it are the ones the conversion writes for them.
func TestElementIDReaderSpellsTheFeatureValueAsTheConversionWritesIt(t *testing.T) {
	sf, index, doc := readerIndex(t, "valued.sysml", "package P { part def R { attribute mass = 3; } }\n")
	graph, err := ToRDF(sf, parser.New(sf).ParseFile())
	if err != nil {
		t.Fatalf("ToRDF: %v", err)
	}
	var expression, membership string
	for _, subject := range graph.Subjects() {
		for _, class := range graph.Objects(subject, rdf.RDFType) {
			switch class.Value {
			case rdf.SysML + "LiteralInteger":
				for _, id := range graph.Objects(subject, rdf.SysML+"elementId") {
					expression = id.Value
				}
			case rdf.SysML + "FeatureValue":
				for _, id := range graph.Objects(subject, rdf.SysML+"elementId") {
					membership = id.Value
				}
			}
		}
	}
	if expression == "" || membership == "" {
		t.Fatalf("the conversion writes expression %q, FeatureValue %q; want both", expression, membership)
	}

	matches := index.LookupQualified("P::R::mass")
	if len(matches) != 1 {
		t.Fatalf("P::R::mass names %d declarations, want 1", len(matches))
	}
	usage, ok := matches[0].Decl.(*ast.Usage)
	if !ok || usage.Value == nil {
		t.Fatalf("P::R::mass declares %T, want a valued usage", matches[0].Decl)
	}
	reader := NewElementIDReader([]ElementIDDocument{doc})
	if got, err := reader.Read(matches[0].DocName, usage.Value, ""); err != nil || got != expression {
		t.Errorf("Read value expression = %q, %v; want %q", got, err, expression)
	}
	if got, err := reader.ReadOwningMembership(matches[0].DocName, usage.Value, ""); err != nil || got != membership {
		t.Errorf("ReadOwningMembership value expression = %q, %v; want %q", got, err, membership)
	}
}
