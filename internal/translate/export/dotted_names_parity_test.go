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

// The parser spells a dotted expression name as a receiver chain or nested chain
// expressions, never as one chained qualified name. Only a receiver-reached call
// (`q.p.f(1)`) owns a chain Feature, at `function`, natively as the conversion writes it.
func TestDottedExpressionNamesOwnAChainFeatureOnlyForAReceiverCallee(t *testing.T) {
	text := `package P {
	calc def F { in x : ScalarValues::Real; return y : ScalarValues::Real = x; }
	part def Pt { attribute a : ScalarValues::Real; attribute b : ScalarValues::Real; calc f : F; }
	part def Q { part p : Pt; }
	part def D {
		part q : Q;
		attribute c = q.p.a;
		attribute d = q.p.f(1);
		attribute e = q->Pt::f(1);
		attribute g = q.p->Pt::f(2);
		attribute h = new Pt(1, 2);
	}
}
`
	sf := source.New("dotted.sysml", []byte(text))
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
	var checked int
	var check func(name string, expr *symbols.Symbol)
	check = func(name string, expr *symbols.Symbol) {
		checked++
		id, err := reader.Read(sf.Name(), expr.Decl, "")
		if err != nil || subjectOf[id].Value == "" {
			t.Fatalf("%s: no written element for %T (%q, %v)", name, expr.Decl, id, err)
		}
		var written []string
		for _, membership := range graph.Objects(subjectOf[id], rdf.SysML+"ownedRelationship") {
			written = append(written, classOf(membership))
		}
		var native []string
		for _, membership := range model.OwnedMemberships(expr) {
			native = append(native, model.MetaclassOf(membership).Name)
		}
		if strings.Join(native, ",") != strings.Join(written, ",") {
			t.Errorf("%s (%s): native owns %v, the conversion writes %v", name, classOf(subjectOf[id]), native, written)
		}
		chain := model.ExpressionChainFeature(expr)
		if (chain != nil) != (name == "d") {
			t.Errorf("%s: native chain Feature %v", name, chain)
		}
		for _, slot := range []string{"function", "targetFeature"} {
			element, _, err := reader.ReadExpressionPart(sf.Name(), expr.Decl, slot)
			owns := err == nil && element != ""
			if owns != (name == "d" && slot == "function") {
				t.Errorf("%s: the conversion owns a chain Feature at %s = %v (%s)", name, slot, owns, element)
			}
			if !owns || chain == nil {
				continue
			}
			var written []string
			for _, link := range graph.Objects(subjectOf[element], rdf.SysML+"chainingFeature") {
				written = append(written, classOf(link)+":"+graph.Objects(link, rdf.SysML+"declaredName")[0].Value)
			}
			var native []string
			for _, link := range model.ChainingFeatures(chain) {
				native = append(native, model.MetaclassOf(link).Name+":"+link.Name)
			}
			if got, want := strings.Join(native, ","), strings.Join(written, ","); got != want || len(native) != 3 {
				t.Errorf("%s: native chains %v, the conversion writes %v", name, native, written)
			}
			if got := classOf(subjectOf[element]); got != "Feature" || model.MetaclassOf(chain).Name != "Feature" {
				t.Errorf("%s: chain Feature written as %s, native %s", name, got, model.MetaclassOf(chain).Name)
			}
		}
		for i, parameter := range model.ExpressionParameters(expr) {
			if operand := model.ValueExpression(parameter); operand != nil {
				check(name+"/in"+string(rune('0'+i)), operand)
			}
		}
	}
	pkg, _ := index.DocumentRoot(sf.Name()).LookupLocal("P")
	d, _ := pkg.Scope.LookupLocal("D")
	for _, name := range []string{"c", "d", "e", "g", "h"} {
		feature, ok := d.Scope.LookupLocal(name)
		if !ok {
			t.Fatalf("no attribute %s", name)
		}
		check(name, model.ValueExpression(feature))
	}
	if checked < 10 {
		t.Errorf("checked only %d expressions", checked)
	}
	ast.Inspect(root, func(node ast.Node) bool {
		var name *ast.QualifiedName
		switch n := node.(type) {
		case *ast.InvocationExpr:
			name = n.Type
		case *ast.ConstructorExpr:
			name = n.Type
		case *ast.FeatureChainExpr:
			name = n.Member
		}
		if name != nil && ast.QualifiedNameHasChain(name) {
			t.Errorf("the parser spelled a chained name %v in a %T", name, node)
		}
		return true
	})
}
