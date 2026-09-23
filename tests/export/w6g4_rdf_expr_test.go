package export_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// turtleOf converts notation to Turtle and parses the result, so a test states
// what the graph says rather than which lines the writer wrote.
func turtleOf(t *testing.T, name, src string) *rdf.Graph {
	t.Helper()
	data, err := convert.Convert(name+".sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph, err := rdf.ParseTurtle(data)
	if err != nil {
		t.Fatalf("parse turtle: %v\n%s", err, data)
	}
	return graph
}

func iri(name string) rdf.Term { return rdf.IRI(name) }

func wantType(t *testing.T, g *rdf.Graph, subject, metaclass string) {
	t.Helper()
	if got := rdf.LocalName(g.Type(iri(subject))); got != metaclass {
		t.Errorf("<%s> is a %q, want %q", subject, got, metaclass)
	}
}

func wantLexical(t *testing.T, g *rdf.Graph, subject, predicate, value string) {
	t.Helper()
	got, ok := g.Lexical(iri(subject), predicate)
	if !ok {
		t.Errorf("<%s> states no %s", subject, predicate)
		return
	}
	if got != value {
		t.Errorf("<%s> %s = %q, want %q", subject, predicate, got, value)
	}
}

// TestExpressionValueIsATree is the point of the mapping: a feature's value is a
// typed structure a consumer can query, not one opaque string.
func TestExpressionValueIsATree(t *testing.T) {
	g := turtleOf(t, "value", `package P {
    attribute a : Integer;
    attribute b : Integer;
    attribute total : Integer = a + b * 2;
}`)
	root := rdf.ExpressionIRI(rdf.ElementIRI("P::total"), "value").Value
	wantType(t, g, root, "OperatorExpression")
	wantLexical(t, g, root, rdf.SysML+"operator", "+")
	wantLexical(t, g, root, rdf.OpenSysML+"sourceText", "a + b * 2")

	args := g.Objects(iri(root), rdf.SysML+"argument")
	if len(args) != 2 {
		t.Fatalf("root has %d arguments, want 2: %v", len(args), args)
	}
	var memberships, returns []rdf.Term
	for _, membership := range g.Objects(iri(root), rdf.SysML+"ownedFeatureMembership") {
		if g.Type(membership) == rdf.SysML+"ReturnParameterMembership" {
			returns = append(returns, membership)
		} else {
			memberships = append(memberships, membership)
		}
	}
	if len(memberships) != 2 {
		t.Fatalf("root has %d parameter memberships, want 2: %v", len(memberships), memberships)
	}
	if len(returns) != 1 {
		t.Fatalf("root has %d return parameter memberships, want 1: %v", len(returns), returns)
	}
	results := g.Objects(iri(root), rdf.SysML+"result")
	if len(results) != 1 || g.Type(results[0]) != rdf.SysML+"Feature" {
		t.Fatalf("root has result %v, want one Feature", results)
	}
	wantLexical(t, g, results[0].Value, rdf.SysML+"direction", "out")
	for _, membership := range memberships {
		wantType(t, g, membership.Value, "ParameterMembership")
		parameters := g.Objects(membership, rdf.SysML+"ownedMemberParameter")
		if len(parameters) != 1 {
			t.Fatalf("parameter membership %s has %d parameters", membership.Value, len(parameters))
		}
		wantType(t, g, parameters[0].Value, "Feature")
		wantLexical(t, g, parameters[0].Value, rdf.SysML+"direction", "in")
		values := g.Objects(parameters[0], rdf.SysML+"ownedMembership")
		if len(values) != 1 {
			t.Fatalf("parameter %s has %d owned memberships", parameters[0].Value, len(values))
		}
		wantType(t, g, values[0].Value, "FeatureValue")
	}
	for _, triple := range g.Triples() {
		if triple.Predicate.Value == rdf.OpenSysML+"argumentIndex" {
			t.Errorf("expression graph still emits sysx:argumentIndex: %v", triple)
		}
	}

	wantType(t, g, args[0].Value, "FeatureReferenceExpression")
	if got := g.Objects(args[0], rdf.SysML+"referent"); len(got) != 1 ||
		got[0].Value != "urn:sysmlv2:element:P__a" {
		t.Errorf("first operand reads %v, want the element P::a", got)
	}
	// The nested operand is a tree of its own, reachable from the root.
	wantType(t, g, args[1].Value, "OperatorExpression")
	wantLexical(t, g, args[1].Value, rdf.SysML+"operator", "*")
	nested := g.Objects(args[1], rdf.SysML+"argument")
	if len(nested) != 2 {
		t.Fatalf("nested operator has %d arguments, want 2", len(nested))
	}
	wantType(t, g, nested[1].Value, "LiteralInteger")
	wantLexical(t, g, nested[1].Value, rdf.SysML+"value", "2")
}

func TestExpressionStructureSurvivesWithoutTextAndArguments(t *testing.T) {
	src := `package P {
    attribute a : Integer;
    attribute total : Integer = a * 2;
}`
	data, err := convert.Convert("structure.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	for _, property := range []string{"sysx:sourceText", "sysx:sourceTail", "sysml:argument", "json:argument"} {
		data = withoutTriples(t, data, property)
	}
	out, err := convert.Convert("structure.ttl", data, convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("structure-only conversion: %v", err)
	}
	if got := strings.Join(strings.Fields(string(out)), " "); !strings.Contains(got, "attribute total : Integer = a * 2;") {
		t.Fatalf("structure-only conversion lost the expression: %s", out)
	}
}

func TestExpressionParameterMembershipAnnotationRestoresOrder(t *testing.T) {
	src := `package P {
    attribute a : Integer;
    attribute b : Integer;
    attribute total : Integer = a - b;
}`
	data, err := convert.Convert("reversed.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	blocks := strings.Split(string(data), "\n\n")
	var first, second int
	for i, block := range blocks {
		switch {
		case strings.HasPrefix(block, "expr:P__total_pvalue_pin0_om\n"):
			first = i
		case strings.HasPrefix(block, "expr:P__total_pvalue_pin1_om\n"):
			second = i
		}
	}
	blocks[first], blocks[second] = blocks[second], blocks[first]
	data = []byte(strings.Join(blocks, "\n\n"))
	for _, property := range []string{"sysx:sourceText", "sysx:sourceTail", "sysml:argument", "json:argument"} {
		data = withoutTriples(t, data, property)
	}
	out, err := convert.Convert("reversed.ttl", data, convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("reversed membership conversion: %v", err)
	}
	if got := strings.Join(strings.Fields(string(out)), " "); !strings.Contains(got, "attribute total : Integer = a - b;") {
		t.Fatalf("annotation did not preserve operand order: %s", out)
	}
}

// Every expression-valued position emits a tree, not only a feature's value:
// a bound, a constraint's condition, a transition's guard and a filter.
func TestExpressionPositionsAllEmitTrees(t *testing.T) {
	g := turtleOf(t, "positions", `package P {
    attribute limit : Integer = 4;
    part def Car {
        attribute pressure : Integer;
        attribute wheels : Integer[1..limit + 1];
        assert constraint { pressure > 0 }
    }
    state def Machine {
        state running;
        transition first running if limit > 1 then running;
    }
    package Filtered {
        filter limit > 2;
    }
}`)
	trees := map[string]string{}
	for _, triple := range g.Triples() {
		if triple.Predicate.Value == rdf.RDFNS+"type" &&
			strings.HasPrefix(triple.Subject.Value, rdf.Expression) {
			trees[strings.TrimPrefix(triple.Subject.Value, rdf.Expression)] =
				rdf.LocalName(triple.Object.Value)
		}
	}
	for _, want := range []struct{ suffix, metaclass string }{
		{"_pvalue", "LiteralInteger"}, // limit = 4
		{"wheels_plowerBound", "LiteralInteger"},
		{"wheels_pupperBound", "OperatorExpression"},
		{"_pguard", "OperatorExpression"},
		{"_pfilter", "OperatorExpression"},
	} {
		found := false
		for name, metaclass := range trees {
			if strings.HasSuffix(name, want.suffix) && metaclass == want.metaclass {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no %s expression in a %s position; the graph states: %v",
				want.metaclass, want.suffix, trees)
		}
	}
	// A constraint member's condition is no longer a position on the assert: the
	// braced expression is a real element owned through its
	// ResultExpressionMembership.
	condition := false
	for _, triple := range g.Triples() {
		if triple.Predicate.Value == rdf.RDFNS+"type" &&
			triple.Object.Value == rdf.SysML+"OperatorExpression" &&
			!strings.HasPrefix(triple.Subject.Value, rdf.Expression) {
			condition = true
		}
	}
	if !condition {
		t.Errorf("the assert's condition is no element; the graph states: %v", trees)
	}
}

// An expression's identity is its owner and its position, so two expressions in
// two positions — and two subexpressions of one — never collide.
func TestExpressionIdentityIsPerPosition(t *testing.T) {
	g := turtleOf(t, "identity", `package P {
    attribute n : Integer;
    attribute a : Integer[n..n] = n + n;
    attribute b : Integer[n..n] = n + n;
}`)
	types := map[string]int{}
	for _, triple := range g.Triples() {
		if triple.Predicate.Value == rdf.RDFNS+"type" &&
			strings.HasPrefix(triple.Subject.Value, rdf.Expression) &&
			triple.Object.Value != rdf.SysML+"Feature" &&
			triple.Object.Value != rdf.SysML+"FeatureValue" &&
			triple.Object.Value != rdf.SysML+"ParameterMembership" &&
			triple.Object.Value != rdf.SysML+"ReturnParameterMembership" &&
			triple.Object.Value != rdf.SysML+"OwningMembership" &&
			triple.Object.Value != rdf.SysML+"Membership" &&
			triple.Object.Value != rdf.SysML+"MultiplicityRange" {
			types[triple.Subject.Value]++
		}
	}
	// Two features, each with a lower bound, an upper bound, a value and the
	// value's two operands: eight expression resources, each stated once.
	if len(types) != 10 {
		t.Errorf("got %d expression resources, want 10: %v", len(types), types)
	}
	for subject, count := range types {
		if count != 1 {
			t.Errorf("<%s> is typed %d times, want once", subject, count)
		}
	}
}

// An expression resource is not a model element: it is not written back as a
// declaration, and its referent is not read as an element's reference.
func TestExpressionResourcesAreNotElements(t *testing.T) {
	const src = `package P {
    attribute a : Integer;
    attribute total : Integer = a + 1;
}`
	turtle, err := convert.Convert("expr.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back, err := convert.Convert("expr.ttl", turtle, convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v\n%s", err, turtle)
	}
	got := strings.Join(strings.Fields(string(back)), " ")
	want := "package P { attribute a : Integer; attribute total : Integer = a + 1; }"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A graph that carries an expression as a literal — every graph this tool wrote
// before the trees, and every foreign graph that writes notation — still reads.
func TestLiteralExpressionsStillDecode(t *testing.T) {
	src := `@prefix sysml: <https://www.omg.org/spec/SysML#> .
@prefix sysx: <urn:opensysml:sysml:> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
@prefix elmt: <urn:sysmlv2:element:> .

elmt:P a sysml:Package ; sysml:declaredName "P" ; sysml:qualifiedName "P" ; sysx:hasBody "true"^^xsd:boolean .
elmt:P__a a sysml:AttributeUsage ; sysml:declaredName "a" ; sysml:qualifiedName "P::a" ;
    sysml:owningNamespace elmt:P ; sysml:type "Integer" .
elmt:P__total a sysml:AttributeUsage ; sysml:declaredName "total" ; sysml:qualifiedName "P::total" ;
    sysml:owningNamespace elmt:P ; sysml:type "Integer" ;
    sysml:lowerBound "1" ; sysml:upperBound "4" ;
    sysml:value "a + 1" .`
	out, err := convert.Convert("literal.ttl", []byte(src), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	got := strings.Join(strings.Fields(string(out)), " ")
	want := "package P { attribute a : Integer; attribute total : Integer[1..4] = a + 1; }"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// An expression stated as structure and no notation is written from the
// structure — the case a foreign graph produces.
func TestForeignExpressionTreeIsWrittenFromItsStructure(t *testing.T) {
	src := `@prefix sysml: <https://www.omg.org/spec/SysML#> .
@prefix sysx: <urn:opensysml:sysml:> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

<urn:uuid:1> a sysml:Package ; sysml:declaredName "P" ; sysml:qualifiedName "P" .
<urn:uuid:2> a sysml:AttributeUsage ; sysml:declaredName "a" ; sysml:qualifiedName "P::a" ;
    sysml:owningNamespace <urn:uuid:1> ; sysml:type "Integer" .
<urn:uuid:3> a sysml:AttributeUsage ; sysml:declaredName "total" ; sysml:qualifiedName "P::total" ;
    sysml:owningNamespace <urn:uuid:1> ; sysml:type "Integer" ;
    sysml:value <urn:uuid:4> .
<urn:uuid:4> a sysml:OperatorExpression ; sysml:operator "+" ;
    sysml:argument <urn:uuid:5>, <urn:uuid:6> .
<urn:uuid:5> a sysml:FeatureReferenceExpression ; sysml:referent <urn:uuid:2> ;
    sysx:argumentIndex "0"^^xsd:integer .
<urn:uuid:6> a sysml:LiteralInteger ; sysml:value "1"^^xsd:integer ; sysx:argumentIndex "1"^^xsd:integer .`
	out, err := convert.Convert("foreign-expr.ttl", []byte(src), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	got := strings.Join(strings.Fields(string(out)), " ")
	want := "package P { attribute a : Integer; attribute total : Integer = a + 1; }"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// An expression the graph states neither notation nor writable structure for is
// a typed unsupported error naming the resource, never a dropped value.
func TestUnsupportedExpressionShapesAreReported(t *testing.T) {
	const head = `@prefix sysml: <https://www.omg.org/spec/SysML#> .
@prefix sysx: <urn:opensysml:sysml:> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

<urn:uuid:1> a sysml:Package ; sysml:declaredName "P" ; sysml:qualifiedName "P" .
<urn:uuid:3> a sysml:AttributeUsage ; sysml:declaredName "total" ; sysml:qualifiedName "P::total" ;
    sysml:owningNamespace <urn:uuid:1> ; sysml:type "Integer" ;
    sysml:value <urn:uuid:4> .
`
	cases := map[string]struct{ triples, note string }{
		"operator with no operator": {
			"<urn:uuid:4> a sysml:OperatorExpression ; sysml:argument <urn:uuid:5> .\n" +
				"<urn:uuid:5> a sysml:LiteralInteger ; sysml:value \"1\"^^xsd:integer .",
			"states the operator it applies",
		},
		"operator with too many operands": {
			"<urn:uuid:4> a sysml:OperatorExpression ; sysml:operator \"+\" ;\n" +
				"    sysml:argument <urn:uuid:5>, <urn:uuid:6>, <urn:uuid:7> .\n" +
				"<urn:uuid:5> a sysml:LiteralInteger ; sysml:value \"1\"^^xsd:integer ; sysx:argumentIndex \"0\"^^xsd:integer .\n" +
				"<urn:uuid:6> a sysml:LiteralInteger ; sysml:value \"2\"^^xsd:integer ; sysx:argumentIndex \"1\"^^xsd:integer .\n" +
				"<urn:uuid:7> a sysml:LiteralInteger ; sysml:value \"3\"^^xsd:integer ; sysx:argumentIndex \"2\"^^xsd:integer .",
			"has no notation",
		},
		"literal with no value": {
			"<urn:uuid:4> a sysml:LiteralInteger .",
			"states the value it evaluates to",
		},
		"feature reference with no referent": {
			"<urn:uuid:4> a sysml:FeatureReferenceExpression .",
			"names the feature it reads",
		},
		"expression with no structure at all": {
			"<urn:uuid:4> a sysml:Expression .",
			"states no notation and no structure",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := convert.Convert("bad.ttl", []byte(head+tc.triples),
				convert.FormatTurtle, convert.FormatSysML)
			if err == nil {
				t.Fatal("expected an error")
			}
			var unsupported *export.UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("error is %T, want *export.UnsupportedError: %v", err, err)
			}
			if !strings.Contains(err.Error(), tc.note) {
				t.Errorf("error %q does not say %q", err, tc.note)
			}
			if !strings.Contains(err.Error(), "urn:uuid:4") {
				t.Errorf("error %q does not name the expression", err)
			}
		})
	}
}

// The trees are additive: what a graph writes back still says what it said, and
// converting that back gives the same graph, for expressions of every shape.
func TestExpressionTreesKeepTheRoundTripExact(t *testing.T) {
	sources := map[string]struct{ src, expression string }{
		"operators": {`package P {
    attribute a : Integer;
    attribute b : Integer;
    attribute c : Integer = a + b * 2 - (a / b);
    attribute d : Boolean = a > b and not (a == b);
    attribute e : Integer = if a > b ? a else b;
}
`, "a + b * 2 - (a / b)"},
		"invocations": {`package P {
    calc def Sum {
        in x : Integer;
        in y : Integer;
        return : Integer = x + y;
    }
    attribute total : Integer = Sum(x = 1, y = 2);
}
`, "Sum(x = 1, y = 2)"},
		"collections": {`package P {
    part def Wheel {
        attribute worn : Boolean;
    }
    part def Car {
        part wheels : Wheel[4];
        attribute worn : Boolean = wheels.?{in w : Wheel; w.worn}->notEmpty();
    }
}
`, "wheels.?{in w : Wheel; w.worn}->notEmpty()"},
		"bounds and guards": {`package P {
    attribute n : Integer;
    attribute many : Integer[1..n + 1];
    state def S {
        state a;
        transition first a if n > 1 then a;
    }
}
`, "n > 1"},
	}
	for name, tc := range sources {
		t.Run(name, func(t *testing.T) {
			first, err := convert.Convert(name+".sysml", []byte(tc.src), convert.FormatSysML, convert.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			back, err := convert.Convert(name+".ttl", first, convert.FormatTurtle, convert.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v\n%s", err, first)
			}
			if !strings.Contains(string(back), tc.expression) {
				t.Errorf("the notation lost %q:\n%s", tc.expression, back)
			}
			second, err := convert.Convert(name+".2.sysml", back, convert.FormatSysML, convert.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle again: %v", err)
			}
			if string(second) != string(first) {
				t.Errorf("round trip changed the graph\n--- first ---\n%s\n--- second ---\n%s", first, second)
			}
		})
	}
}

// A binding head states its connector ends through standard ownership.
func TestBindingEndsAreStatedAsStructure(t *testing.T) {
	g := turtleOf(t, "ends", `package P {
    port def Bus;
    part def Car {
        port left : Bus;
        port right : Bus;
        connect left to right;
    }
    interface def I {
        end supplier : Bus;
        end consumer : Bus;
        flow supplier.out to consumer.in;
    }
    part def Bus2 { attribute out : Integer; attribute in : Integer; }
}`)
	ends := map[string][]string{}
	for _, triple := range g.Triples() {
		if triple.Predicate.Value == rdf.SysML+"connectorEnd" {
			ends[triple.Subject.Value] = append(ends[triple.Subject.Value], triple.Object.Value)
		}
	}
	if len(ends) != 2 {
		t.Fatalf("got %d binding heads with stated ends, want 2: %v", len(ends), ends)
	}
	for subject, related := range ends {
		if len(related) != 2 {
			t.Errorf("<%s> relates %d features, want 2", subject, len(related))
			continue
		}
		// The head keeps its notation, and every end is an owned end feature.
		if _, ok := g.Lexical(iri(subject), rdf.OpenSysML+"sourceText"); !ok {
			t.Errorf("<%s> states no source text", subject)
		}
		wantType(t, g, related[0], "ReferenceUsage")
		wantType(t, g, related[1], "ReferenceUsage")
		if !g.BoolValue(iri(related[0]), rdf.SysML+"isEnd") || !g.BoolValue(iri(related[1]), rdf.SysML+"isEnd") {
			t.Errorf("<%s> does not mark both ends with sysml:isEnd", subject)
		}
	}
	// A connect end names the port it connects; a flow end reaches through one.
	connectEnd := rdf.Expression + rdf.ExpressionNodeID("P__Car___402", "end0")
	wantType(t, g, connectEnd, "ReferenceUsage")
	relationships := g.Objects(iri(connectEnd), rdf.SysML+"ownedReferenceSubsetting")
	if len(relationships) != 1 {
		t.Fatalf("the first connect end has %d ReferenceSubsetting relationships, want 1", len(relationships))
	}
	if got := g.Objects(relationships[0], rdf.SysML+"referencedFeature"); len(got) != 1 ||
		got[0].Value != "urn:sysmlv2:element:P__Car__left" {
		t.Errorf("the first connect end references %v, want the port P::Car::left", got)
	}
	for _, subject := range g.Subjects() {
		if g.Type(subject) != rdf.SysML+"FlowUsage" {
			continue
		}
		flowEnds := g.Objects(subject, rdf.SysML+"connectorEnd")
		if len(flowEnds) != 2 {
			t.Errorf("flow usage <%s> has %d connector ends, want 2", subject.Value, len(flowEnds))
			continue
		}
		for _, end := range flowEnds {
			wantType(t, g, end.Value, "ReferenceUsage")
		}
	}
}
