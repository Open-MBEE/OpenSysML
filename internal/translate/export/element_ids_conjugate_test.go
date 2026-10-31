package export

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// The ids the reader answers for the conjugated port definition a port
// definition implies, its owning membership and its PortConjugation are the
// ones the conversion writes for them.
func TestElementIDReaderSpellsTheConjugateAsTheConversionWritesIt(t *testing.T) {
	sf, index, doc := readerIndex(t, "ports.sysml", "package P { port def Q { attribute x; } }\n")
	graph, err := ToRDF(sf, parser.New(sf).ParseFile())
	if err != nil {
		t.Fatalf("ToRDF: %v", err)
	}
	written := map[string]string{}
	for _, subject := range graph.Subjects() {
		for _, class := range graph.Objects(subject, rdf.RDFType) {
			switch class.Value {
			case rdf.SysML + "ConjugatedPortDefinition", rdf.SysML + "PortConjugation":
				for _, id := range graph.Objects(subject, rdf.SysML+"elementId") {
					written[class.Value] = id.Value
				}
			}
		}
	}
	if len(written) != 2 {
		t.Fatalf("the conversion writes %v, want a ConjugatedPortDefinition and a PortConjugation", written)
	}
	conjugate := written[rdf.SysML+"ConjugatedPortDefinition"]
	var membership string
	for _, subject := range graph.Subjects() {
		for _, member := range graph.Objects(subject, rdf.SysML+"ownedMemberElement") {
			if rdf.LocalName(member.Value) == conjugate {
				for _, id := range graph.Objects(subject, rdf.SysML+"elementId") {
					membership = id.Value
				}
			}
		}
	}
	if membership == "" {
		t.Fatalf("the conversion writes no owning membership for %s", conjugate)
	}

	matches := index.LookupQualified("P::Q")
	if len(matches) != 1 {
		t.Fatalf("P::Q names %d declarations, want 1", len(matches))
	}
	sym, ok := matches[0].Scope.LookupLocal("~Q")
	if !ok || !sym.IsConjugatedPortDefinition() {
		t.Fatalf("P::Q::'~Q' = %v, want the implied conjugate", sym)
	}
	if _, ok := symbols.ImpliedDeclOf(sym); !ok {
		t.Fatalf("the conjugate's declaration is %T, want an ImpliedDecl", sym.Decl)
	}
	reader := NewElementIDReader([]ElementIDDocument{doc})
	if got, err := reader.Read(sym.DocName, sym.Decl, "P::Q::~Q"); err != nil || got != conjugate {
		t.Errorf("Read conjugate = %q, %v; want %q", got, err, conjugate)
	}
	if got, err := reader.ReadOwningMembership(sym.DocName, sym.Decl, "P::Q::~Q"); err != nil || got != membership {
		t.Errorf("ReadOwningMembership conjugate = %q, %v; want %q", got, err, membership)
	}
	if got, err := reader.ReadRelationship(sym.DocName, sym.Decl); err != nil || got != written[rdf.SysML+"PortConjugation"] {
		t.Errorf("ReadRelationship conjugation = %q, %v; want %q", got, err, written[rdf.SysML+"PortConjugation"])
	}
}
