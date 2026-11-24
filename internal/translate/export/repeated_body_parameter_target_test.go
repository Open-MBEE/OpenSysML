package export

import (
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// A body parameter naming a target again (`in a : A, A :> g, g, h`) owns one relationship per name,
// and the symbol table spells each by the id the conversion writes, in the order it seats them.
func TestRepeatedBodyParameterTargetSymbolIDsMatchTheConversion(t *testing.T) {
	sf := source.New("body.sysml", []byte(`package P {
	part def A;
	part g : A;
	part h : A;
	calc def F { in xs : A[*]; xs->collect { in a : A, A :> g, g, h; a } }
}
`))
	root := parser.New(sf).ParseFile()
	graph, res, encoders, err := modelToRDF([]ModelDocument{{File: sf, Root: root}}, IDQualifiedName)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := identityIDsOf(res, encoders, []bool{false})
	if err != nil {
		t.Fatal(err)
	}
	const parameter = "P__F___401_pa0_pin0"
	var written []string
	for _, rel := range graph.Objects(rdf.IRI(rdf.Expression+parameter), rdf.SysML+"ownedRelationship") {
		if id := rdf.LocalName(rel.Value); !strings.HasSuffix(id, "_om") {
			written = append(written, id)
		}
	}
	want := []string{parameter + "_ft0", parameter + "_ft1", parameter + "_ss0", parameter + "_ss2", parameter + "_ss1"}
	if !slices.Equal(written, want) {
		t.Fatalf("the conversion writes %v, want %v", written, want)
	}
	model := encoders[0].ids.model
	symbolIDs := NewSymbolIDs(ids, res, model)
	var read []string
	seen := map[*symbols.Symbol]bool{}
	var walk func(sym *symbols.Symbol)
	walk = func(sym *symbols.Symbol) {
		if sym == nil || seen[sym] {
			return
		}
		seen[sym] = true
		for _, rel := range model.OwnedRelationships(sym) {
			if id, ok := symbolIDs.Of(rel); ok && strings.HasPrefix(id, parameter+"_") && !strings.HasSuffix(id, "_om") {
				read = append(read, id)
			}
			related, _ := model.ReflectiveElements(rel, "ownedRelatedElement")
			for _, element := range related {
				walk(element)
			}
		}
	}
	walk(model.RootNamespace(sf.Name()))
	if !slices.Equal(read, want) {
		t.Errorf("the symbol table reads %v, want %v", read, want)
	}
}
