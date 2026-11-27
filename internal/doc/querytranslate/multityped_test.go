package querytranslate

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// A feature whose own types disagree on what its values are has no kind of its
// own, in either order of the types; one whose types agree keeps theirs.
func TestClassifySettlesAMultiTypedFeatureOnlyWhenItsTypesAgree(t *testing.T) {
	index := libs.NewModelIndex()
	p := parser.New(source.New("multityped.sysml", []byte(`
package Typed {
	part def Thing;
	part def Other;
	part def Holder {
		attribute textThenFlag : ScalarValues::String, ScalarValues::Boolean;
		attribute flagThenText : ScalarValues::Boolean, ScalarValues::String;
		attribute numberThenThing : ScalarValues::Integer, Thing;
		attribute twoNumbers : ScalarValues::Integer, ScalarValues::Real;
		attribute twoThings : Thing, Other;
	}
}`)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse fixture: %v", p.Diagnostics)
	}
	index.AddDocument("multityped.sysml", root)
	index.ExpandWildcardImports()
	model := semantics.NewModel(resolve.New(index))
	for name, want := range map[string]valueKind{
		"textThenFlag":    kindUnknown,
		"flagThenText":    kindUnknown,
		"numberThenThing": kindUnknown,
		"twoNumbers":      kindNumber,
		"twoThings":       kindElement,
	} {
		feature := index.LookupUnique("Typed::Holder::" + name)
		if feature == nil {
			t.Fatalf("%s: not in the fixture", name)
		}
		if got := len(model.FeatureTypes(feature)); got != 2 {
			t.Fatalf("%s: %d types, want 2", name, got)
		}
		table := NewTable(index, model)
		table.features[name] = []*symbols.Symbol{feature}
		if got := table.kind(name); got != want {
			t.Errorf("kind(%s) = %d, want %d", name, got, want)
		}
	}
}
