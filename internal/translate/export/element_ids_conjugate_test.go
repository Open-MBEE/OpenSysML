package export

import (
	"strings"
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

// A port definition declared by a short name alone has a conjugate named by
// that short name: `~<s>`, no declared name, addressed as `P::'~s'`.
func TestConversionNamesAShortNamedPortsConjugateByItsShortName(t *testing.T) {
	sf, _, _ := readerIndex(t, "short.sysml", "package P { port def <s>; }\n")
	graph, err := ToRDF(sf, parser.New(sf).ParseFile())
	if err != nil {
		t.Fatalf("ToRDF: %v", err)
	}
	for _, subject := range graph.Subjects() {
		for _, class := range graph.Objects(subject, rdf.RDFType) {
			if class.Value != rdf.SysML+"ConjugatedPortDefinition" {
				continue
			}
			if got := graph.Objects(subject, rdf.SysML+"declaredName"); len(got) != 0 {
				t.Errorf("declaredName = %v, want none for a short-named original", got)
			}
			if got := graph.Objects(subject, rdf.SysML+"declaredShortName"); len(got) != 1 || got[0].Value != "~s" {
				t.Errorf("declaredShortName = %v, want ~s", got)
			}
			// The original's address may be positional (#1805); the conjugate's
			// segment under it is its effective name.
			address := append(graph.Objects(subject, rdf.SysML+"qualifiedName"),
				graph.Objects(subject, rdf.OpenSysML+xPositionalName)...)
			if len(address) != 1 || !strings.HasSuffix(address[0].Value, "::'~s'") {
				t.Errorf("address = %v, want one ending in ::'~s'", address)
			}
			return
		}
	}
	t.Fatal("the conversion writes no ConjugatedPortDefinition")
}

// A short-named port definition's graph converts back: the reverse converter
// checks the conjugate's short name against the original's.
func TestShortNamedPortDefinitionRoundTripsThroughRDF(t *testing.T) {
	sf, _, _ := readerIndex(t, "short_rt.sysml", "package P { port def <s>; }\n")
	graph, err := ToRDF(sf, parser.New(sf).ParseFile())
	if err != nil {
		t.Fatalf("ToRDF: %v", err)
	}
	out, err := ToSysML(graph)
	if err != nil {
		t.Fatalf("ToSysML: %v", err)
	}
	if !strings.Contains(string(out), "port def <s>") {
		t.Errorf("round trip lost the port definition:\n%s", out)
	}
}
