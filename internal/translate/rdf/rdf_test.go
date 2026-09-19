package rdf

import (
	"strings"
	"testing"
)

func TestGraphDeduplicates(t *testing.T) {
	g := NewGraph()
	subject := IRI("urn:x")
	g.Add(subject, IRI("urn:p"), String("v"))
	g.Add(subject, IRI("urn:p"), String("v"))
	if g.Len() != 1 {
		t.Errorf("got %d triples, want 1", g.Len())
	}
	g.Add(subject, IRI("urn:p"), String("w"))
	if g.Len() != 2 {
		t.Errorf("got %d triples, want 2", g.Len())
	}
}

func TestGraphKeepsSubjectOrder(t *testing.T) {
	g := NewGraph()
	for _, name := range []string{"c", "a", "b"} {
		g.Add(IRI("urn:"+name), IRI("urn:p"), String("v"))
	}
	var got []string
	for _, subject := range g.Subjects() {
		got = append(got, subject.Value)
	}
	want := []string{"urn:c", "urn:a", "urn:b"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestGraphLookups(t *testing.T) {
	g := NewGraph()
	subject := IRI("urn:x")
	g.Add(subject, IRI(RDFType), SysMLTerm("PartUsage"))
	g.Add(subject, SysMLTerm("declaredName"), String("wheel"))
	g.Add(subject, SysMLTerm("isOrdered"), Bool(true))
	g.Add(subject, SysMLTerm("memberIndex"), Int(3))

	if got := g.Type(subject); got != SysML+"PartUsage" {
		t.Errorf("Type = %q", got)
	}
	if got, ok := g.Lexical(subject, SysML+"declaredName"); !ok || got != "wheel" {
		t.Errorf("Lexical = %q, %v", got, ok)
	}
	if !g.BoolValue(subject, SysML+"isOrdered") {
		t.Error("BoolValue = false, want true")
	}
	if g.BoolValue(subject, SysML+"missing") {
		t.Error("BoolValue of an absent property should be false")
	}
	if got, ok := g.Lexical(subject, SysML+"memberIndex"); !ok || got != "3" {
		t.Errorf("Lexical of an integer = %q, %v", got, ok)
	}
	if _, ok := g.Object(IRI("urn:absent"), SysML+"declaredName"); ok {
		t.Error("Object of an absent subject should not be found")
	}
	if got := g.Objects(subject, SysML+"declaredName"); len(got) != 1 {
		t.Errorf("Objects returned %d terms, want 1", len(got))
	}
}

func TestElementIRIRoundTrip(t *testing.T) {
	for _, qname := range []string{
		"Vehicle",
		"Demo::Vehicle",
		"A::B::C",
		"has space",
		"has#hash",
		"has<angle>",
		"has\"quote",
		"has%percent",
	} {
		term := ElementIRI(qname)
		if !term.IsIRI() {
			t.Fatalf("%q did not produce an IRI", qname)
		}
		if !strings.HasPrefix(term.Value, Element) {
			t.Fatalf("%q: IRI %q is not in the element namespace", qname, term.Value)
		}
		got, ok := DecodeElementID(strings.TrimPrefix(term.Value, Element))
		if !ok {
			t.Fatalf("%q: the id of IRI %q does not decode", qname, term.Value)
		}
		if got != qname {
			t.Errorf("%q round tripped as %q", qname, got)
		}
	}
}

func TestElementIRIEscapesTurtleDelimiters(t *testing.T) {
	// A name containing a character that would end an IRI reference has to be
	// escaped, or the document it is written into will not parse.
	term := ElementIRI("a>b c")
	for _, bad := range []string{">", " ", "<", `"`} {
		if strings.Contains(term.Value, bad) {
			t.Errorf("IRI %q still contains %q", term.Value, bad)
		}
	}
}

func TestLocalName(t *testing.T) {
	cases := map[string]string{
		SysML + "PartUsage":        "PartUsage",
		"https://example.com/a/b":  "b",
		"urn:sysmlv2:element:X__Y": "X__Y",
		"plain":                    "plain",
	}
	for iri, want := range cases {
		if got := LocalName(iri); got != want {
			t.Errorf("LocalName(%q) = %q, want %q", iri, got, want)
		}
	}
}

func TestWriteTurtleShape(t *testing.T) {
	g := NewGraph()
	subject := ElementIRI("P::Q")
	g.Add(subject, IRI(RDFType), SysMLTerm("PartDefinition"))
	g.Add(subject, SysMLTerm("declaredName"), String("Q"))
	g.Add(subject, SysMLTerm("isAbstract"), Bool(true))
	g.Add(subject, OpenSysMLTerm("memberIndex"), Int(0))

	out := string(WriteTurtle(g))
	for _, want := range []string{
		"@prefix sysml: <https://www.omg.org/spec/SysML#> .",
		"@prefix elmt: <urn:sysmlv2:element:> .",
		"elmt:P__Q",
		"a sysml:PartDefinition ;",
		`sysml:declaredName "Q" ;`,
		`sysml:isAbstract "true"^^xsd:boolean`,
		`sysx:memberIndex "0"^^xsd:integer`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}
	if !strings.HasSuffix(strings.TrimSpace(out), ".") {
		t.Errorf("the last statement is not terminated:\n%s", out)
	}
}

func TestWriteTurtleWritesTypeFirst(t *testing.T) {
	g := NewGraph()
	subject := IRI("urn:x")
	g.Add(subject, SysMLTerm("declaredName"), String("x"))
	g.Add(subject, IRI(RDFType), SysMLTerm("Package"))
	out := string(WriteTurtle(g))
	typeAt := strings.Index(out, "a sysml:Package")
	nameAt := strings.Index(out, "declaredName")
	if typeAt < 0 || nameAt < 0 || typeAt > nameAt {
		t.Errorf("rdf:type should be written first:\n%s", out)
	}
}

func TestTurtleRoundTrip(t *testing.T) {
	g := NewGraph()
	subject := ElementIRI("Demo::Part")
	g.Add(subject, IRI(RDFType), SysMLTerm("PartUsage"))
	g.Add(subject, SysMLTerm("declaredName"), String("Part"))
	g.Add(subject, SysMLTerm("owningNamespace"), ElementIRI("Demo"))
	g.Add(subject, SysMLTerm("isOrdered"), Bool(false))
	g.Add(subject, OpenSysMLTerm("memberIndex"), Int(7))
	g.Add(subject, OpenSysMLTerm("sourceText"), String("line one\nline two \"quoted\"\ttabbed"))
	g.Add(subject, SysMLTerm("body"), String(`a \ backslash`))
	g.Add(ElementIRI("Demo"), IRI(RDFType), SysMLTerm("Package"))

	data := WriteTurtle(g)
	parsed, err := ParseTurtle(data)
	if err != nil {
		t.Fatalf("parse: %v\n%s", err, data)
	}
	if parsed.Len() != g.Len() {
		t.Errorf("got %d triples, want %d\n%s", parsed.Len(), g.Len(), data)
	}
	again := WriteTurtle(parsed)
	if string(again) != string(data) {
		t.Errorf("rewriting changed the document\n--- first ---\n%s\n--- second ---\n%s", data, again)
	}
}

func TestParseTurtleForms(t *testing.T) {
	src := `@prefix sysml: <https://www.omg.org/spec/SysML#> .
@prefix elmt: <urn:sysmlv2:element:> .
PREFIX ex: <https://example.com/>

elmt:A a sysml:Package ;
    sysml:declaredName "A" ;
    sysml:body """a
multiline body""" ;
    ex:label "hello"@en ;
    sysml:count "4"^^<http://www.w3.org/2001/XMLSchema#integer> .

elmt:B a sysml:PartDefinition , sysml:Thing ;
    sysml:owningNamespace elmt:A .`
	g, err := ParseTurtle([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	a := IRI(Element + "A")
	if got := g.Type(a); got != SysML+"Package" {
		t.Errorf("type of A = %q", got)
	}
	if got, ok := g.Lexical(a, SysML+"body"); !ok || got != "a\nmultiline body" {
		t.Errorf("multiline body = %q, %v", got, ok)
	}
	label, ok := g.Object(a, "https://example.com/label")
	if !ok || label.Lang != "en" || label.Value != "hello" {
		t.Errorf("language literal = %+v, %v", label, ok)
	}
	count, ok := g.Object(a, SysML+"count")
	if !ok || count.Datatype != XSD+"integer" || count.Value != "4" {
		t.Errorf("typed literal = %+v, %v", count, ok)
	}
	if got := len(g.Objects(IRI(Element+"B"), RDFType)); got != 2 {
		t.Errorf("B has %d types, want 2", got)
	}
}

func TestParseTurtleEscapes(t *testing.T) {
	src := `<urn:x> <urn:p> "quote \" backslash \\ tab \t newline \n backspace \b feed \f unicode \u00e9" .`
	g, err := ParseTurtle([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got, _ := g.Lexical(IRI("urn:x"), "urn:p")
	want := "quote \" backslash \\ tab \t newline \n backspace \b feed \f unicode é"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseTurtleRejects(t *testing.T) {
	cases := map[string]string{
		"blank node subject":  `_:b <urn:p> "v" .`,
		"blank node object":   `<urn:x> <urn:p> _:b .`,
		"anonymous node":      `<urn:x> <urn:p> [ <urn:q> "v" ] .`,
		"collection":          `<urn:x> <urn:p> ( <urn:a> <urn:b> ) .`,
		"undefined prefix":    `ex:x <urn:p> "v" .`,
		"missing terminator":  `<urn:x> <urn:p> "v"`,
		"unterminated string": `<urn:x> <urn:p> "v .`,
		"unterminated iri":    `<urn:x <urn:p> "v" .`,
		"no predicate":        `<urn:x> .`,
		"no object":           `<urn:x> <urn:p> .`,
		"bare number":         `<urn:x> <urn:p> 4 .`,
		"bare boolean":        `<urn:x> <urn:p> true .`,
		"bad escape":          `<urn:x> <urn:p> "\q" .`,
		"empty prefix decl":   `@prefix .`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseTurtle([]byte(src)); err == nil {
				t.Errorf("expected an error for %s", src)
			}
		})
	}
}

func TestParseErrorReportsLine(t *testing.T) {
	src := "<urn:a> <urn:p> \"v\" .\n<urn:b> <urn:p> _:x .\n"
	_, err := ParseTurtle([]byte(src))
	if err == nil {
		t.Fatal("expected an error")
	}
	parseErr, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("got %T, want *ParseError", err)
	}
	if parseErr.Line != 2 {
		t.Errorf("Line = %d, want 2", parseErr.Line)
	}
	if !strings.Contains(err.Error(), "2") {
		t.Errorf("message should mention the line: %v", err)
	}
}

func TestParseTurtleBase(t *testing.T) {
	src := "@base <https://example.com/> .\n<a> <p> <b> .\n"
	g, err := ParseTurtle([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, ok := g.Object(IRI("https://example.com/a"), "https://example.com/p"); !ok {
		t.Errorf("base was not applied:\n%s", WriteTurtle(g))
	}
}

func TestParseTurtleComments(t *testing.T) {
	src := `# a leading comment
<urn:x> <urn:p> "v" . # a trailing comment
# a closing comment`
	g, err := ParseTurtle([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if g.Len() != 1 {
		t.Errorf("got %d triples, want 1", g.Len())
	}
}

func TestParseEmptyDocument(t *testing.T) {
	g, err := ParseTurtle([]byte("  \n# nothing here\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if g.Len() != 0 {
		t.Errorf("got %d triples, want 0", g.Len())
	}
}

func TestTermPredicates(t *testing.T) {
	if !IRI("urn:x").IsIRI() || IRI("urn:x").IsLiteral() {
		t.Error("an IRI should be an IRI and not a literal")
	}
	if !String("v").IsLiteral() || String("v").IsIRI() {
		t.Error("a string should be a literal and not an IRI")
	}
	if Bool(true).Datatype != XSD+"boolean" {
		t.Error("a boolean should carry the xsd:boolean datatype")
	}
	if Int(2).Datatype != XSD+"integer" {
		t.Error("an integer should carry the xsd:integer datatype")
	}
}

// A value with newlines and quotes in any arrangement must be escaped so that
// it neither closes the literal early nor spills onto another line.
func TestMultilineLiteralsSurviveRoundTripOnOneLine(t *testing.T) {
	for _, value := range []string{
		"\"a\" +\n    \"b\"",
		"line\n\"",
		"line\n\"\"\"inside\"\"\"",
		"trailing pair\n\"\"",
	} {
		g := NewGraph()
		g.Add(ElementIRI("A"), IRI(SysML+"value"), String(value))
		turtle := WriteTurtle(g)
		for _, line := range strings.Split(string(turtle), "\n") {
			if strings.Contains(line, "sysml:value") && !strings.HasSuffix(line, `" .`) {
				t.Errorf("value %q spills over its line:\n%s", value, turtle)
			}
		}
		parsed, err := ParseTurtle(turtle)
		if err != nil {
			t.Errorf("value %q does not parse back: %v\n%s", value, err, turtle)
			continue
		}
		got, ok := parsed.Lexical(ElementIRI("A"), SysML+"value")
		if !ok || got != value {
			t.Errorf("value %q came back as %q (ok=%v)", value, got, ok)
		}
	}
}

// A short Turtle literal may not carry a raw control character; each one is
// written as its ECHAR or as a \u escape and must read back unchanged.
func TestControlCharactersAreEscaped(t *testing.T) {
	const value = "bs\b ff\f nul\x00 esc\x1b del\x7f tab\t"
	const lexical = `"bs\b ff\f nul\u0000 esc\u001B del\u007F tab\t"`
	g := NewGraph()
	g.Add(ElementIRI("A"), IRI(SysML+"value"), String(value))
	turtle := WriteTurtle(g)
	if !strings.Contains(string(turtle), lexical) {
		t.Fatalf("want the literal %s in:\n%s", lexical, turtle)
	}
	parsed, err := ParseTurtle(turtle)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got, ok := parsed.Lexical(ElementIRI("A"), SysML+"value"); !ok || got != value {
		t.Errorf("value %q came back as %q (ok=%v)", value, got, ok)
	}
}

// Two bindings can both match one IRI. Map iteration is randomised, so the
// abbreviation must be chosen rather than whichever is reached first.
func TestPrefixChoiceIsDeterministic(t *testing.T) {
	const doc = `@prefix sysml: <https://www.omg.org/spec/SysML#> .
@prefix a: <urn:sysmlv2:> .
@prefix elmt: <urn:sysmlv2:element:> .
elmt:A a sysml:Package .
`
	want := ""
	for i := 0; i < 50; i++ {
		g, err := ParseTurtle([]byte(doc))
		if err != nil {
			t.Fatal(err)
		}
		got := string(WriteTurtle(g))
		if want == "" {
			want = got
			continue
		}
		if got != want {
			t.Fatalf("writing the same graph twice differed:\n%s\n---\n%s", want, got)
		}
	}
	// The longest matching namespace is the most specific abbreviation.
	if !strings.Contains(want, "elmt:A") {
		t.Errorf("want the most specific prefix elmt:, got:\n%s", want)
	}
}

// `PREFIX`/`BASE` are keywords only as whole words: a prefixed name that starts
// with the same letters is a term, and reading it as a directive would reject a
// valid document.
func TestPrefixLikeLabelIsNotADirective(t *testing.T) {
	const doc = `@prefix base: <urn:x:> .
@prefix prefixed: <urn:y:> .
@prefix sysml: <https://www.omg.org/spec/SysML#> .
base:Thing a sysml:Package .
prefixed:Other a sysml:Package .
`
	g, err := ParseTurtle([]byte(doc))
	if err != nil {
		t.Fatalf("a prefixed name beginning with a keyword was rejected: %v", err)
	}
	if got := g.Len(); got != 2 {
		t.Errorf("want 2 triples, got %d", got)
	}
}
