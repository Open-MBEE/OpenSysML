package export

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// The reference form of satisfy and verify (`satisfy r by sys;`, `verify r;`) subsets its
// requirement by a ReferenceSubsetting on both ledgers; a plain `:>` stays a Subsetting.
func TestReferenceFormSubsettingReadsAsTheConversionWritesIt(t *testing.T) {
	text := `package P {
	part def Sys;
	requirement def R;
	requirement r : R;
	part sys : Sys;
	part x : Sys {
		satisfy r by sys;
		verify r;
		part y :> sys;
	}
	verification def V { objective { verify r; } }
}
`
	sf := source.New("reference.sysml", []byte(text))
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
	x, _ := pkg.Scope.LookupLocal("x")
	v, _ := pkg.Scope.LookupLocal("V")
	var objective *symbols.Symbol
	for _, member := range v.Scope.AllMembers() {
		if usage, ok := member.Decl.(*ast.Usage); ok && usage.Kind == ast.UsageObjective {
			objective = member
		}
	}
	owners := map[string]*symbols.Symbol{"satisfy": nil, "verify": nil, "y": nil, "objective verify": nil}
	for _, member := range x.Scope.AllMembers() {
		usage, _ := member.Decl.(*ast.Usage)
		switch {
		case usage != nil && usage.IsVerifiedRequirement():
			owners["verify"] = member
		case usage != nil && usage.Kind == ast.UsageSatisfy:
			owners["satisfy"] = member
		case member.Name == "y":
			owners["y"] = member
		}
	}
	for _, member := range objective.Scope.AllMembers() {
		if usage, ok := member.Decl.(*ast.Usage); ok && usage.IsVerifiedRequirement() {
			owners["objective verify"] = member
		}
	}
	for name, owner := range owners {
		if owner == nil {
			t.Fatalf("%s: owner not found", name)
		}
		want := "ReferenceSubsetting"
		if name == "y" {
			want = "Subsetting"
		}
		var rel *symbols.Symbol
		for _, candidate := range model.ImplicitRelationships(owner) {
			if candidate.Implicit != nil && candidate.Implicit.Kind == ast.RelSubsets {
				rel = candidate
			}
		}
		if rel == nil {
			t.Fatalf("%s: no subsets relationship among %v", name, model.ImplicitRelationships(owner))
		}
		ownerID, err := reader.Read(sf.Name(), owner.Decl, "")
		if err != nil || subjectOf[ownerID].Value == "" {
			t.Fatalf("%s: owner not written (%q, %v)", name, ownerID, err)
		}
		var written []string
		for _, object := range graph.Objects(subjectOf[ownerID], rdf.SysML+"ownedRelationship") {
			if class := classOf(object); class == "Subsetting" || class == "ReferenceSubsetting" {
				written = append(written, class)
			}
		}
		if got := model.MetaclassOf(rel).Name; got != want || len(written) != 1 || written[0] != want {
			t.Errorf("%s: %s natively, %v written, want %s", name, got, written, want)
		}
		referenced, _ := model.ReflectiveElements(rel, "referencedFeature")
		owned, _ := model.ReflectiveElements(owner, "ownedReferenceSubsetting")
		if isReference := want == "ReferenceSubsetting"; (len(referenced) == 1) != isReference || (len(owned) == 1 && owned[0] == rel) != isReference {
			t.Errorf("%s: referencedFeature %v, ownedReferenceSubsetting %v, want present %v", name, referenced, owned, isReference)
		}
	}
}
