package export

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// A subject's `by a.b.c` chain owns one FeatureChaining per written link whether or not the
// link resolves, as the conversion writes them (recordSatisfySubjectValue), each at referent.fcN.
func TestSatisfySubjectChainWithAnUnresolvedLinkOwnsEveryChaining(t *testing.T) {
	text := `package P {
	part def Sys { part sub : Sys; }
	requirement def R { subject s : Sys; }
	part sys : Sys { satisfy requirement r : R by sys.nothing.sub; }
}
`
	sf := source.New("satisfy-unresolved.sysml", []byte(text))
	p := parser.New(sf)
	root := p.ParseFile()
	index := libs.NewModelIndex()
	index.AddDocument(sf.Name(), root)
	index.ExpandWildcardImports()
	resolver := resolve.New(index)
	model := semantics.NewModel(resolver)
	resolver.ResolveDocument(sf.Name(), root)
	graph, err := ToRDF(sf, root)
	if err != nil {
		t.Fatalf("ToRDF: %v", err)
	}
	subjectOf := map[string]rdf.Term{}
	for _, subject := range graph.Subjects() {
		for _, id := range graph.Objects(subject, rdf.SysML+"elementId") {
			subjectOf[id.Value] = subject
		}
	}
	reader := NewElementIDReader([]ElementIDDocument{{File: sf, Root: root, ParseDiags: p.Diagnostics}})
	pkg, _ := index.DocumentRoot(sf.Name()).LookupLocal("P")
	sys, _ := pkg.Scope.LookupLocal("sys")
	satisfy, _ := sys.Scope.LookupLocal("r")
	value := model.ValueExpression(model.SubjectParameterOf(satisfy))
	referent, _ := model.ReflectiveElements(value, "referent")
	if len(referent) != 1 {
		t.Fatalf("referent = %v, want the chain Feature", referent)
	}
	element, _, err := reader.ReadExpressionPart(sf.Name(), value.Decl, "referent")
	if err != nil {
		t.Fatal(err)
	}
	chainings, _ := model.ReflectiveElements(referent[0], "ownedFeatureChaining")
	written := graph.Objects(subjectOf[element], rdf.SysML+"ownedFeatureChaining")
	if len(chainings) != 3 || len(written) != 3 {
		t.Fatalf("native owns %d FeatureChainings, the conversion writes %d, want 3 and 3", len(chainings), len(written))
	}
	for i := range chainings {
		part, _, err := reader.ReadExpressionPart(sf.Name(), value.Decl, "referent.fc"+string(rune('0'+i)))
		if err != nil || part != rdf.LocalName(written[i].Value) {
			t.Errorf("chaining %d id %q (%v), written %v", i, part, err, written[i])
		}
	}
}
