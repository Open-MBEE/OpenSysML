package export

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// A satisfy's `by` subject binds its value as the conversion writes it: a plain name as a
// FeatureReferenceExpression naming it by a Membership, a chain `a.b` as a
// FeatureReferenceExpression owning a chain Feature at `referent` (SysML.xtext
// SatisfactionReferenceExpression) — same metaclass, owned memberships, chainings and ids.
func TestSatisfySubjectValueReadsAsTheConversionWritesIt(t *testing.T) {
	text := `package P {
	part def Sys { part sub : Sys; }
	requirement def R { subject s : Sys; }
	part sys : Sys {
		satisfy requirement r : R by sys;
		satisfy requirement r3 : R by sys.sub;
	}
}
`
	sf := source.New("satisfy.sysml", []byte(text))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse: %v", p.Diagnostics)
	}
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
	classOf := func(term rdf.Term) string {
		for _, class := range graph.Objects(term, rdf.RDFType) {
			return strings.TrimPrefix(class.Value, rdf.SysML)
		}
		return "?"
	}
	reader := NewElementIDReader([]ElementIDDocument{{File: sf, Root: root, ParseDiags: p.Diagnostics}})
	pkg, _ := index.DocumentRoot(sf.Name()).LookupLocal("P")
	sys, _ := pkg.Scope.LookupLocal("sys")
	for name, chained := range map[string]bool{"r": false, "r3": true} {
		satisfy, _ := sys.Scope.LookupLocal(name)
		subject := model.SubjectParameterOf(satisfy)
		value := model.ValueExpression(subject)
		if subject == nil || value == nil {
			t.Fatalf("%s: no subject (%v) or value (%v)", name, subject, value)
		}
		id, err := reader.Read(sf.Name(), value.Decl, "")
		if err != nil || subjectOf[id].Value == "" {
			t.Fatalf("%s: no written value for %T (%q, %v)", name, value.Decl, id, err)
		}
		if got, want := model.MetaclassOf(value).Name, classOf(subjectOf[id]); got != want || want != "FeatureReferenceExpression" {
			t.Errorf("%s: value is %s natively, %s written", name, got, want)
		}
		var written, native []string
		for _, membership := range graph.Objects(subjectOf[id], rdf.SysML+"ownedRelationship") {
			written = append(written, classOf(membership))
		}
		for _, membership := range model.OwnedMemberships(value) {
			native = append(native, model.MetaclassOf(membership).Name)
		}
		if strings.Join(native, ",") != strings.Join(written, ",") {
			t.Errorf("%s: native owns %v, the conversion writes %v", name, native, written)
		}
		referent, _ := model.ReflectiveElements(value, "referent")
		chain := model.ExpressionChainFeature(value)
		if (chain != nil) != chained || len(referent) != 1 || (chained && referent[0] != chain) {
			t.Errorf("%s: native chain Feature %v, referent %v, want chained %v", name, chain, referent, chained)
		}
		element, _, err := reader.ReadExpressionPart(sf.Name(), value.Decl, "referent")
		if (err == nil && element != "") != chained {
			t.Errorf("%s: the conversion owns a chain Feature at referent = %q (%v), want %v", name, element, err, chained)
		}
		if !chained {
			if got := graph.Objects(subjectOf[id], rdf.SysML+"referent"); len(got) != 1 || classOf(got[0]) != "PartUsage" || model.MetaclassOf(referent[0]).Name != "PartUsage" {
				t.Errorf("%s: referent written %v, native %v, want the part sys", name, got, referent)
			}
			continue
		}
		var writtenLinks, nativeLinks []string
		for _, link := range graph.Objects(subjectOf[element], rdf.SysML+"chainingFeature") {
			writtenLinks = append(writtenLinks, classOf(link)+":"+graph.Objects(link, rdf.SysML+"declaredName")[0].Value)
		}
		for _, link := range model.ChainingFeatures(chain) {
			nativeLinks = append(nativeLinks, model.MetaclassOf(link).Name+":"+link.Name)
		}
		if got, want := strings.Join(nativeLinks, ","), strings.Join(writtenLinks, ","); got != want || len(nativeLinks) != 2 {
			t.Errorf("%s: native chains %v, the conversion writes %v", name, nativeLinks, writtenLinks)
		}
		chainings, _ := model.ReflectiveElements(chain, "ownedFeatureChaining")
		writtenChainings := graph.Objects(subjectOf[element], rdf.SysML+"ownedFeatureChaining")
		if len(chainings) != len(writtenChainings) || len(chainings) != 2 {
			t.Errorf("%s: native owns %d FeatureChainings, the conversion writes %d", name, len(chainings), len(writtenChainings))
		}
		for i, chaining := range chainings {
			part, _, err := reader.ReadExpressionPart(sf.Name(), value.Decl, "referent.fc"+string(rune('0'+i)))
			if err != nil || i >= len(writtenChainings) || part != rdf.LocalName(writtenChainings[i].Value) {
				t.Errorf("%s: chaining %d id %q (%v), written %v", name, i, part, err, writtenChainings)
			}
			if model.MetaclassOf(chaining).Name != "FeatureChaining" {
				t.Errorf("%s: chaining %d is %s natively", name, i, model.MetaclassOf(chaining).Name)
			}
		}
		if result := model.ExpressionResult(value); result == nil || symbols.IsAbstract(result) {
			t.Errorf("%s: native result %v", name, result)
		}
	}
}
