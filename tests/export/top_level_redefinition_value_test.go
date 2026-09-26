package export_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

func TestTopLevelRedefinitionRetainsValueInOwnedAttribute(t *testing.T) {
	src := `package P {
	attribute def Tempo {
		attribute operative;
	}
	part def Block {
		attribute tempo : Tempo;
	}
	part def Writer :> Block {
		attribute :>> tempo = Tempo::operative;
	}
}
`
	turtle, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatalf("parse turtle: %v", err)
	}
	writer := elementNamed(t, graph, "P::Writer")
	owned := graph.Objects(writer, rdf.SysML+"ownedMember")
	if len(owned) != 1 {
		t.Fatalf("Writer has %d owned members, want one", len(owned))
	}
	attribute := owned[0]
	if got := rdf.LocalName(graph.Type(attribute)); got != "AttributeUsage" {
		t.Fatalf("redefined member has type %q, want AttributeUsage", got)
	}
	if got, ok := graph.Object(attribute, rdf.SysML+"owningNamespace"); !ok || got != writer {
		t.Errorf("attribute owningNamespace = %v, want %v", got, writer)
	}
	membership, ok := graph.Object(attribute, rdf.SysML+"owningMembership")
	if !ok {
		t.Fatal("redefined attribute has no owning membership")
	}
	if got := graph.Objects(writer, rdf.SysML+"ownedMembership"); len(got) == 0 || got[0] != membership {
		t.Errorf("Writer ownedMembership = %v, want membership %v", got, membership)
	}
	wantRedefined := elementNamed(t, graph, "P::Block::tempo")
	if got := graph.Objects(attribute, rdf.SysML+"redefines"); len(got) != 1 || got[0] != wantRedefined {
		t.Errorf("attribute redefines = %v, want Block::tempo %v", got, wantRedefined)
	}
	value, ok := graph.Object(attribute, rdf.SysML+"value")
	if !ok {
		t.Fatal("redefined attribute has no value")
	}
	if got, ok := graph.Lexical(value, rdf.OpenSysML+"sourceText"); !ok || got != "Tempo::operative" {
		t.Errorf("value sourceText = %q, present=%v, want Tempo::operative", got, ok)
	}
}
