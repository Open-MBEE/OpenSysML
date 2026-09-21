package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// setTensorModel values collections from expressions and builds rank-three and
// rank-four tensors: the RDF graph carries the expression for each feature.
const setTensorModel = `package P {
	private import ScalarValues::*;
	private import Collections::*;
	private import CollectionFunctions::*;
	private import Quantities::*;
	private import MeasurementReferences::*;
	private import SI::*;
	attribute s : Set { :>> elements = (3, 1, 2, 2, 3); }
	attribute e : Set { :>> elements = (); }
	attribute nested : Set { :>> elements = (s, e, s); }
	attribute u : UniqueCollection { :>> elements = (2, 3, 2, 1); }
	attribute kv1 : KeyValuePair { :>> key = (1); :>> val = (2); }
	attribute kv2 : KeyValuePair { :>> key = (3); :>> val = (4); }
	attribute m : Map { :>> elements = (kv2, kv1, kv2); }
	attribute sameAsS : Set { :>> elements = (2, 3, 1); }
	attribute notS : Set { :>> elements = (1, 2); }
	attribute hyperRef : TensorMeasurementReference {
		:>> dimensions = (2, 1, 2, 2);
		:>> mRefs = (Pa, Pa, Pa, Pa, Pa, Pa, Pa, Pa);
	}
	attribute hyper : TensorQuantityValue = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0), hyperRef);
	attribute hyperCorner : Real = hyper#(2, 1, 1, 2);
	attribute cubeRef : TensorMeasurementReference {
		:>> dimensions = (2, 2, 2);
		:>> mRefs = (Pa, Pa, Pa, Pa, Pa, Pa, Pa, Pa);
	}
	attribute cube : TensorQuantityValue = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0), cubeRef);
	attribute corner : Real = cube#(2, 1, 2);
}
`

// A set or a tensor has no literal form in RDF: the graph states the expression
// the feature is written with, and the runtime evaluates it again after the
// hop. The trip must therefore be exact, with and without the source text.
func TestSetAndTensorValuesRoundTripAsExpressions(t *testing.T) {
	turtle := roundTripsExactly(t, setTensorModel)
	text := string(turtle)
	for _, want := range []string{
		`sysml:redefines "elements"`,
		"a sysml:OperatorExpression ;\n    sysx:sourceText \"(3, 1, 2, 2, 3)\"",
		`sysx:sourceText "()"`,
		"a sysml:InvocationExpression ;\n    sysx:sourceText \"TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0), cubeRef)\"",
		`sysml:function "TensorCalculations::["`,
		`sysx:sourceText "cube#(2, 1, 2)"`,
		`sysx:sourceText "(2, 1, 2, 2)"`,
		`sysx:sourceText "hyper#(2, 1, 1, 2)"`,
		`sysml:type "Set"`,
		`sysml:type "UniqueCollection"`,
		`sysml:type "Map"`,
		`sysml:type "TensorMeasurementReference"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("graph lacks %q:\n%s", want, text)
		}
	}
	for _, never := range []string{"Set{", "Tensor(", "xsd:integer\", \"", "urn:opensysml:set", "urn:opensysml:tensor"} {
		if strings.Contains(text, never) {
			t.Errorf("graph states an evaluated value %q, which the mapping does not define:\n%s", never, text)
		}
	}

	stripped := withoutSourceText(t, turtle)
	back, err := convert.Convert("m.ttl", stripped, convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation from the expression trees alone: %v", err)
	}
	for _, want := range []string{
		"redefines elements = (3, 1, 2, 2, 3);",
		"redefines elements = null;",
		"redefines elements = (s, e, s);",
		"redefines dimensions = (2, 2, 2);",
		"redefines dimensions = (2, 1, 2, 2);",
		"= TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0), cubeRef);",
		"= cube#(2, 1, 2);",
		"= hyper#(2, 1, 1, 2);",
		"redefines elements = (2, 3, 2, 1);",
		"redefines elements = (kv2, kv1, kv2);",
	} {
		if !strings.Contains(string(back), want) {
			t.Errorf("the expression trees alone should spell %q\n--- notation ---\n%s", want, back)
		}
	}
	again, err := convert.Convert("m.sysml", []byte(back), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle again: %v", err)
	}
	if lost, gained := tripleSetDiff(t, stripped, withoutSourceText(t, again)); len(lost)+len(gained) > 0 {
		t.Errorf("the expression trees alone changed the graph\n--- lost ---\n%s\n--- gained ---\n%s",
			strings.Join(lost, "\n"), strings.Join(gained, "\n"))
	}
}

func TestSetAndTensorGraphsAreStandardShaped(t *testing.T) {
	turtle := withoutSourceText(t, idTurtle(t, setTensorModel))
	graph, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{"OperatorExpression": true, "LiteralInteger": true, "LiteralRational": true, "InvocationExpression": true, "FeatureReferenceExpression": true, "NullExpression": true}
	var roots []rdf.Term
	for _, tr := range graph.Triples() {
		if tr.Predicate.Value == rdf.SysML+"value" && tr.Subject.IsIRI() {
			roots = append(roots, tr.Object)
		}
	}
	seen := map[rdf.Term]bool{}
	for len(roots) > 0 {
		n := roots[0]
		roots = roots[1:]
		if !n.IsIRI() || seen[n] {
			continue
		}
		typ := rdf.LocalName(graph.Type(n))
		if !strings.Contains(n.Value, "expr:") {
			continue
		}
		if !allowed[typ] {
			t.Errorf("expression node %s has nonstandard class %s", n.Value, typ)
			continue
		}
		seen[n] = true
		for _, p := range []string{rdf.SysML + "argument", rdf.SysML + "value", rdf.SysML + "referent"} {
			roots = append(roots, graph.Objects(n, p)...)
		}
	}
	for _, tr := range graph.Triples() {
		if !seen[tr.Subject] {
			continue
		}
		name := strings.ToLower(rdf.LocalName(tr.Predicate.Value))
		for _, forbidden := range []string{"set", "tensor", "member", "component", "rank", "shape"} {
			if strings.Contains(name, forbidden) {
				t.Errorf("expression graph uses value-specific predicate %s", tr.Predicate.Value)
			}
		}
	}
	control := idTurtle(t, `package P {
	private import ScalarValues::*;
	private import Collections::*;
	attribute a : Real = 1.0 + 2.0;
	attribute array : Array { :>> dimensions = (2, 2); :>> elements = (1, 2, 3, 4); }
}
`)
	controlGraph := rdfMustParse(t, control)
	controlPredicates := map[string]bool{}
	for _, tr := range controlGraph.Triples() {
		if strings.HasPrefix(tr.Predicate.Value, rdf.OpenSysML) && strings.Contains(tr.Subject.Value, "expr:") {
			controlPredicates[tr.Predicate.Value] = true
		}
	}
	for _, tr := range graph.Triples() {
		if seen[tr.Subject] && strings.HasPrefix(tr.Predicate.Value, rdf.OpenSysML) && !controlPredicates[tr.Predicate.Value] {
			t.Errorf("set/tensor expression graph uses sysx predicate absent from control: %s", tr.Predicate.Value)
		}
	}
}

func rdfMustParse(t *testing.T, turtle []byte) *rdf.Graph {
	t.Helper()
	g, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestSetAndTensorStructuralPredicatesCarryTheRoundTrip(t *testing.T) {
	stripped := withoutSourceText(t, idTurtle(t, setTensorModel))
	for _, tc := range []struct {
		pred, spelling string
		degrades       bool
	}{
		{"sysml:operator", "(3, 1, 2, 2, 3)", true},
		{"sysx:argumentIndex", "(3, 1, 2, 2, 3)", false},
		{"sysml:function", "TensorCalculations::'['(", true},
		{"sysml:referent", "cubeRef", true},
		{"sysml:argument", "TensorCalculations::'['(", true},
	} {
		mutated := withoutTriples(t, stripped, tc.pred)
		if tc.pred == "sysml:argument" {
			mutated = withoutTriples(t, mutated, "json:argument")
		}
		back, err := convert.Convert("m.ttl", mutated, convert.FormatTurtle, convert.FormatSysML)
		if tc.degrades {
			if err == nil && strings.Contains(string(back), tc.spelling) {
				t.Errorf("removing %s preserved %q", tc.pred, tc.spelling)
			}
		} else if err != nil || !strings.Contains(string(back), tc.spelling) {
			t.Errorf("removing non-load-bearing %s unexpectedly degraded notation: %v\n%s", tc.pred, err, back)
		}
	}
}

func runtimeModel(t *testing.T, notation string) (*runtime.Context, *symbols.Scope) {
	t.Helper()
	file := parser.New(source.New("<test>", []byte(notation))).ParseFile()
	idx := libs.NewModelIndex()
	idx.AddDocument("<test>", file)
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	model.SetArgumentTyper(passes.NewArgumentTyper(resolver, model))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("P")
	if !ok || pkg.Scope == nil {
		t.Fatal("package P not indexed")
	}
	return runtime.NewContext(runtime.NewModel(model, resolver), 100000), pkg.Scope
}

func evalExportExpr(t *testing.T, ctx *runtime.Context, scope *symbols.Scope, expr string) runtime.Value {
	t.Helper()
	p := parser.New(source.New("<expr>", []byte(expr)))
	node := p.ParseExpression()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse %q: %v", expr, p.Diagnostics)
	}
	value, err := ctx.EvalWithScope(node, scope)
	if err != nil {
		t.Fatalf("evaluate %q: %v", expr, err)
	}
	return value
}

func TestSetOrderAndTensorShapeSurviveTheHop(t *testing.T) {
	originalCtx, originalScope := runtimeModel(t, setTensorModel)
	back := toNotation(t, withoutSourceText(t, idTurtle(t, setTensorModel)))
	hoppedCtx, hoppedScope := runtimeModel(t, back)
	for _, expr := range []string{"s == sameAsS", "s == notS"} {
		before := evalExportExpr(t, originalCtx, originalScope, expr)
		after := evalExportExpr(t, hoppedCtx, hoppedScope, expr)
		if runtime.FormatTraceValue(before) != runtime.FormatTraceValue(after) {
			t.Errorf("%s changed: %s -> %s", expr, runtime.FormatTraceValue(before), runtime.FormatTraceValue(after))
		}
	}
	for _, expr := range []string{"s->size()", "m.elements->size()", "cube#(2, 1, 2)", "hyper#(2, 1, 1, 2)", "hyper"} {
		before := evalExportExpr(t, originalCtx, originalScope, expr)
		after := evalExportExpr(t, hoppedCtx, hoppedScope, expr)
		if runtime.FormatTraceValue(before) != runtime.FormatTraceValue(after) {
			t.Errorf("%s changed: %s -> %s", expr, runtime.FormatTraceValue(before), runtime.FormatTraceValue(after))
		}
		if expr == "hyper" && !strings.HasPrefix(runtime.FormatTraceValue(after), "Tensor(2, 1, 2, 2)") {
			t.Errorf("hyper has wrong shape: %s", runtime.FormatTraceValue(after))
		}
	}
	if strings.Contains(string(idTurtle(t, setTensorModel)), `sysx:sourceText "(3, 1, 2, 2, 3)"`) == false {
		t.Fatal("set source text missing")
	}
	if strings.Contains(string(idTurtle(t, setTensorModel)), `sysx:sourceText "(2, 3, 1)"`) == false {
		t.Fatal("same-order set source text missing")
	}
}
