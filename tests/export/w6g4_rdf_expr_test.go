package export_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
)

// turtleOf converts notation to Turtle and parses the result, so a test states
// what the graph says rather than which lines the writer wrote.
func turtleOf(t *testing.T, name, src string) *rdf.Graph {
	t.Helper()
	data, err := export.Convert(name+".sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
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
	// Order is stated as well as written: an argument carries its index.
	wantLexical(t, g, args[0].Value, rdf.OpenSysML+"argumentIndex", "0")
	wantLexical(t, g, args[1].Value, rdf.OpenSysML+"argumentIndex", "1")

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
		{"_pcondition", "OperatorExpression"},
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
			strings.HasPrefix(triple.Subject.Value, rdf.Expression) {
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
	turtle, err := export.Convert("expr.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back, err := export.Convert("expr.ttl", turtle, export.FormatTurtle, export.FormatSysML)
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
	out, err := export.Convert("literal.ttl", []byte(src), export.FormatTurtle, export.FormatSysML)
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
	out, err := export.Convert("foreign-expr.ttl", []byte(src), export.FormatTurtle, export.FormatSysML)
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
			_, err := export.Convert("bad.ttl", []byte(head+tc.triples),
				export.FormatTurtle, export.FormatSysML)
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
			first, err := export.Convert(name+".sysml", []byte(tc.src), export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			back, err := export.Convert(name+".ttl", first, export.FormatTurtle, export.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v\n%s", err, first)
			}
			if !strings.Contains(string(back), tc.expression) {
				t.Errorf("the notation lost %q:\n%s", tc.expression, back)
			}
			second, err := export.Convert(name+".2.sysml", back, export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle again: %v", err)
			}
			if string(second) != string(first) {
				t.Errorf("round trip changed the graph\n--- first ---\n%s\n--- second ---\n%s", first, second)
			}
		})
	}
}

// A binding head is kept as source text, but the features it relates are stated
// as structure beside it, so a consumer reads the ends without parsing notation.
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
		if triple.Predicate.Value == rdf.OpenSysML+"relatedFeature" {
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
		// The head keeps its notation, and every end says where it is written.
		if _, ok := g.Lexical(iri(subject), rdf.OpenSysML+"sourceText"); !ok {
			t.Errorf("<%s> states no source text", subject)
		}
		wantLexical(t, g, related[0], rdf.OpenSysML+"endIndex", "0")
		wantLexical(t, g, related[1], rdf.OpenSysML+"endIndex", "1")
	}
	// A connect end names the port it connects; a flow end reaches through one.
	connectEnd := rdf.Expression + rdf.ExpressionNodeID("P__Car___402", "end0")
	wantType(t, g, connectEnd, "FeatureReferenceExpression")
	if got := g.Objects(iri(connectEnd), rdf.SysML+"referent"); len(got) != 1 ||
		got[0].Value != "urn:sysmlv2:element:P__Car__left" {
		t.Errorf("the first connect end reads %v, want the port P::Car::left", got)
	}
	source := rdf.Expression + rdf.ExpressionNodeID("P__I___402", "flowSource")
	wantType(t, g, source, "FeatureChainExpression")
	wantLexical(t, g, source, rdf.OpenSysML+"endRole", "source")
	wantLexical(t, g, rdf.Expression+rdf.ExpressionNodeID("P__I___402", "flowTarget"),
		rdf.OpenSysML+"endRole", "target")
}
