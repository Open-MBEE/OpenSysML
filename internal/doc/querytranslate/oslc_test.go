package querytranslate

import (
	"maps"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/query"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

func mustParse(t *testing.T, text string) Query {
	t.Helper()
	q, err := ParseOSLC(text, nil)
	if err != nil {
		t.Fatalf("ParseOSLC(%q): %v", text, err)
	}
	return q
}

func kindOf(t *testing.T, err error, want ErrorKind) *Error {
	t.Helper()
	got, ok := err.(*Error)
	if !ok || got.Kind != want {
		t.Fatalf("error = %#v, want kind %d", err, want)
	}
	return got
}

func TestParseOSLCOperatorsAndValues(t *testing.T) {
	q := mustParse(t, `sysml:name="wheel" and sysx:multiplicityLower>=2 and sysml:type in [sysml:PartUsage, "x"]`)
	terms := q.Where.(Conjunction).Terms
	if len(terms) != 3 {
		t.Fatalf("where = %#v", q.Where)
	}
	if c := terms[0].(Comparison); c.Property != query.PropertyName || c.Values[0] != "wheel" {
		t.Fatalf("string term = %#v", c)
	}
	if c := terms[1].(Comparison); c.Operator != GreaterEqual || c.Property != query.PropertyMultiplicityLower {
		t.Fatalf("ordered term = %#v", c)
	}
	if c := terms[2].(Comparison); c.Operator != In || c.Values[0] != "PartUsage" || c.Values[1] != "x" {
		t.Fatalf("in term = %#v", c)
	}
}

func TestParseOSLCTypeTermsAndInstanceTerms(t *testing.T) {
	if tt := mustParse(t, `rdf:type=sysml:PartUsage`).Where.(TypeTest); !tt.Exact || tt.Types[0] != "PartUsage" || tt.Inverse {
		t.Fatalf("type test = %#v", tt)
	}
	if tt := mustParse(t, `rdf:type!="PartUsage"`).Where.(TypeTest); !tt.Inverse {
		t.Fatalf("negated type test = %#v", tt)
	}
	if tt := mustParse(t, `rdf:type in [sysml:PartUsage, sysml:PartDefinition]`).Where.(TypeTest); len(tt.Types) != 2 {
		t.Fatalf("type list = %#v", tt)
	}
	id := rdf.EncodeElementID("Demo::Vehicle")
	if in := mustParse(t, `rdf:type=elmt:`+id).Where.(Instances); in.Elements[0] != "Demo::Vehicle" {
		t.Fatalf("instances = %#v", in)
	}
	if in := mustParse(t, `rdf:type=<`+rdf.Element+id+`>`).Where.(Instances); in.Elements[0] != "Demo::Vehicle" {
		t.Fatalf("instances by IRI = %#v", in)
	}
	if _, err := ParseOSLC(`rdf:type in [elmt:`+id+`, sysml:PartUsage]`, nil); err == nil {
		t.Fatal("mixed metaclass and element list unexpectedly parsed")
	}
	if _, err := ParseOSLC(`rdf:type>sysml:PartUsage`, nil); err == nil {
		t.Fatal("ordered rdf:type unexpectedly parsed")
	} else {
		kindOf(t, err, ErrUnorderedProperty)
	}
}

func TestParseOSLCScopedTermsAndNestedSelection(t *testing.T) {
	q := mustParse(t, `sysml:owner{sysml:name="x" and rdf:type=sysml:Package}`)
	scoped := q.Where.(Scoped)
	if scoped.Property != query.PropertyOwner {
		t.Fatalf("scoped = %#v", scoped)
	}
	if inner := scoped.Inner.(Conjunction); len(inner.Terms) != 2 {
		t.Fatalf("scoped inner = %#v", inner)
	}
	q, err := ParseParameters(`oslc.select=sysml:name,sysml:owner{sysml:name,sysml:qualifiedName}&oslc.orderBy=-sysml:owner{sysml:name}`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cols := q.Columns(); strings.Join(cols, " ") != "name owner.name owner.qualifiedName" {
		t.Fatalf("columns = %v", cols)
	}
	if q.SpellingOf("owner.name") != "sysml:owner{sysml:name}" || q.SpellingOf("owner.qualifiedName") != "sysml:owner{sysml:qualifiedName}" {
		t.Fatalf("spelling = %#v", q.Spelling)
	}
	if len(q.OrderBy) != 1 || q.OrderBy[0].Property != "owner.name" || !q.OrderBy[0].Desc {
		t.Fatalf("order = %#v", q.OrderBy)
	}
}

func TestParseOSLCWildcardAndSearchTerms(t *testing.T) {
	q, err := ParseParameters(`oslc.select=*,sysml:owner{*}&oslc.searchTerms=%22wheel%22,%22axle%22`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Select) != 2 || q.Select[0].Property != SelectAll || q.Select[1].Nested[0].Property != SelectAll {
		t.Fatalf("select = %#v", q.Select)
	}
	if len(q.SearchTerms) != 2 || q.SearchTerms[1] != "axle" {
		t.Fatalf("search terms = %#v", q.SearchTerms)
	}
	for _, text := range []string{`oslc.searchTerms=wheel`, `oslc.searchTerms=`, `oslc.searchTerms=%22%22`} {
		if _, err := ParseOSLC(text, nil); err == nil {
			t.Errorf("%q unexpectedly parsed", text)
		} else {
			kindOf(t, err, ErrMalformed)
		}
	}
}

func TestParseOSLCParametersAndPrefixes(t *testing.T) {
	q, err := ParseParameters(`oslc.prefix=ex%3D%3Curn%3Aexample%3A%3E&oslc.where=sysml%3Aname%3D%22x%22&oslc.select=sysml%3Aname%2Crdf%3Atype&oslc.orderBy=-sysml%3Aname`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cols := q.Columns(); len(cols) != 2 || cols[0] != query.PropertyName || cols[1] != query.PropertyType {
		t.Fatalf("select = %#v", q.Select)
	}
	if len(q.OrderBy) != 1 || !q.OrderBy[0].Desc {
		t.Fatalf("order = %#v", q.OrderBy)
	}
	if q.SpellingOf(query.PropertyName) != "sysml:name" || q.SpellingOf(query.PropertyType) != "rdf:type" {
		t.Fatalf("spelling = %#v", q.Spelling)
	}
	if q.SpellingOf(query.PropertyOwner) != query.PropertyOwner {
		t.Fatalf("spelling of an unasked property = %q", q.SpellingOf(query.PropertyOwner))
	}
}

func TestParseOSLCRefusesAPropertySelectedTwice(t *testing.T) {
	const bound = `oslc.prefix=s%3D%3Chttps://www.omg.org/spec/SysML%23%3E`
	for _, tt := range []struct{ selection, message string }{
		{`sysml%3Aname%2Cs%3Aname`, `"sysml:name" and "s:name" are the same property`},
		{`sysml%3Aname%2Csysml%3Aname`, `names "sysml:name" twice`},
	} {
		_, err := ParseParameters(bound+`&oslc.select=`+tt.selection, nil)
		if err == nil {
			t.Fatalf("%s unexpectedly parsed", tt.selection)
		}
		if got := kindOf(t, err, ErrMalformed); !strings.Contains(got.Message, tt.message) {
			t.Fatalf("%s error message = %q, want it to contain %q", tt.selection, got.Message, tt.message)
		}
	}
	q, err := ParseParameters(bound+`&oslc.select=sysml%3Aname%2Cs%3Aowner`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if q.SpellingOf(query.PropertyName) != "sysml:name" || q.SpellingOf(query.PropertyOwner) != "s:owner" {
		t.Fatalf("spelling = %#v", q.Spelling)
	}
}

func TestParseOSLCRefusesWhatTheLanguageDoesNotDefine(t *testing.T) {
	tests := []struct {
		text string
		kind ErrorKind
	}{
		{`sysml:name="x" or sysml:name="y"`, ErrUnsupportedDisjunction},
		{`oslc.orderBy=*`, ErrMalformed},
		{`oslc.where=*=x`, ErrMalformed},
		{`oslc.properties=*`, ErrMalformed},
		{`sysml:name="x"@en`, ErrUnsupportedLiteral},
		{`rdf:type!=elmt:Demo`, ErrUnsupportedInstanceTerm},
		{`sysml:id="x"`, ErrUnknownProperty},
		// Text past the compound term is not dropped.
		{`sysml:name="a"} and sysml:name="b"`, ErrMalformed},
		{`oslc.where=sysml:name%3D%22a%22%7D+and+sysml:name%3D%22b%22`, ErrMalformed},
	}
	for _, tt := range tests {
		_, err := ParseOSLC(tt.text, nil)
		if err == nil {
			t.Errorf("%q unexpectedly parsed", tt.text)
			continue
		}
		if got, ok := err.(*Error); !ok || got.Kind != tt.kind {
			t.Errorf("%q error = %#v, want kind %d", tt.text, err, tt.kind)
		}
	}
}

func TestParseOSLCAllowsInfinityForOrderedMultiplicity(t *testing.T) {
	for _, text := range []string{`sysx:multiplicityUpper>=*`, `sysx:multiplicityUpper=*`, `sysx:multiplicityUpper in [*, 1]`} {
		c := mustParse(t, text).Where.(Comparison)
		if c.Values[0] != "*" {
			t.Fatalf("ParseOSLC(%q) = %#v", text, c)
		}
	}
}

func TestParseOSLCRefusesTheWildcardValueElsewhere(t *testing.T) {
	for _, text := range []string{`sysml:name=*`, `sysml:name in ["a", *]`, `rdf:type=*`} {
		_, err := ParseOSLC(text, nil)
		if got := kindOf(t, err, ErrUnsupportedWildcard); !strings.Contains(got.Message, "sysx:multiplicityUpper") {
			t.Fatalf("ParseOSLC(%q) message = %q, want the bounds named", text, got.Message)
		}
	}
	// A quoted "*" is a literal, not the wildcard.
	if c := mustParse(t, `sysml:name="*"`).Where.(Comparison); c.Values[0] != "*" {
		t.Fatalf("quoted literal = %#v", c)
	}
}

func TestOSLCPropertyNamespacesFollowTheMetamodel(t *testing.T) {
	// A property the metamodel defines is spelled sysml:, one OpenSysML derives sysx:.
	_, err := ParseOSLC(`sysml:multiplicityUpper=1`, nil)
	if got := kindOf(t, err, ErrUnknownProperty); !strings.Contains(got.Message, "sysx:multiplicityUpper") {
		t.Fatalf("message = %q, want the sysx: spelling offered", got.Message)
	}
	_, err = ParseOSLC(`sysx:name="x"`, nil)
	if got := kindOf(t, err, ErrUnknownProperty); !strings.Contains(got.Message, "sysml:name") {
		t.Fatalf("message = %q, want the sysml: spelling offered", got.Message)
	}
	_, err = ParseOSLC(`sysml:nmae="x"`, nil)
	if got := kindOf(t, err, ErrUnknownProperty); !strings.Contains(got.Message, "sysml:name") {
		t.Fatalf("message = %q, want the nearest spelling", got.Message)
	}
	for _, name := range query.PropertyNames() {
		if name == query.PropertyID || name == query.PropertyType {
			continue
		}
		prefix := "sysml:"
		if name == query.PropertyMultiplicityLower || name == query.PropertyMultiplicityUpper {
			prefix = "sysx:"
		}
		if _, err := ParseOSLC(prefix+name+`="x"`, nil); err != nil {
			t.Errorf("%s%s: %v", prefix, name, err)
		}
	}
}

func TestParseOSLCReducesURIAndPrefixedValuesAlike(t *testing.T) {
	for _, text := range []string{`sysml:type=sysml:PartUsage`, `sysml:type=<https://www.omg.org/spec/SysML#PartUsage>`} {
		if c := mustParse(t, text).Where.(Comparison); c.Values[0] != "PartUsage" {
			t.Fatalf("ParseOSLC(%q) = %#v, want the value PartUsage", text, c)
		}
	}
	if c := mustParse(t, `sysml:name=<urn:example:battery>`).Where.(Comparison); c.Values[0] != "urn:example:battery" {
		t.Fatalf("value = %#v, want a foreign IRI kept whole", c.Values)
	}
}

func TestParseOSLCRefusesParametersItDoesNotRead(t *testing.T) {
	for _, text := range []string{
		`oslc.wher=sysml:name%3D%22x%22`, `oslc.where=sysml:name%3D%22x%22&oslc.where=sysml:name%3D%22y%22`,
		`oslc.where=`, `oslc.select=`, `oslc.prefix=`, `oslc.properties=sysml:name`,
	} {
		_, err := ParseOSLC(text, nil)
		if err == nil {
			t.Errorf("%q unexpectedly parsed", text)
			continue
		}
		if got, ok := err.(*Error); !ok || got.Kind != ErrMalformed {
			t.Errorf("%q error = %#v, want ErrMalformed", text, err)
		}
	}
	_, err := ParseOSLC(`oslc.pagin=true&oslc.where=sysml:name%3D%22x%22`, nil)
	if got := kindOf(t, err, ErrMalformed); !strings.Contains(got.Message, "ignores oslc.pageSize, oslc.paging") {
		t.Errorf("error message = %q, want the ignored parameters named", got.Message)
	}
}

// OSLC Query 3.0 lets a server ignore the paging parameters (QUERY-59, QUERY-62),
// which a stock client sends by default; refusing them is stricter than the spec.
func TestParseOSLCIgnoresThePagingParameters(t *testing.T) {
	for _, text := range []string{
		`oslc.paging=true&oslc.where=sysml:name%3D%22x%22`,
		`oslc.pageSize=50&oslc.paging=true&oslc.paging=false&oslc.where=sysml:name%3D%22x%22`,
		`oslc.paging=&oslc.pageSize=&oslc.where=sysml:name%3D%22x%22`,
	} {
		q, err := ParseOSLC(text, nil)
		if err != nil {
			t.Fatalf("ParseOSLC(%q): %v", text, err)
		}
		if c := q.Where.(Comparison); c.Values[0] != "x" {
			t.Fatalf("ParseOSLC(%q) where = %#v", text, q.Where)
		}
	}
	if got := strings.Join(IgnoredParameterNames(), " "); got != "oslc.pageSize oslc.paging" {
		t.Fatalf("IgnoredParameterNames() = %q", got)
	}
}

// sort_term ::= scoped_sort_terms | ("+" | "-") identifier and scoped_sort_terms ::=
// identifier "{" sort_terms "}": signs sit inside the braces, which hold several terms.
func TestParseOSLCScopedSortTermsAreSignedInside(t *testing.T) {
	q, err := ParseParameters(`oslc.orderBy=sysml:owner%7B%2Bsysml:name,-sysml:qualifiedName%7D,-sysml:name`, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []OrderTerm{{Property: "owner.name"}, {Property: "owner.qualifiedName", Desc: true}, {Property: "name", Desc: true}}
	if !slices.Equal(q.OrderBy, want) {
		t.Fatalf("order = %#v, want %#v", q.OrderBy, want)
	}
	// The specification's own example, percent-encoded as it gives it, parses up
	// to its unbound prefix.
	_, err = ParseParameters(`oslc.orderBy=dcterms%3Acreator%7B%2Bfoaf%3Aname%7D%2C-dcterms%3Acreated`, nil)
	if got := kindOf(t, err, ErrMalformed); !strings.Contains(got.Message, `"dcterms" is unbound`) {
		t.Fatalf("error = %q, want the unbound prefix, not the sign", got.Message)
	}
	// Nested scopes carry their signs at the leaf; an outer sign reaches the
	// unsigned leaves; a sign on both sides is refused.
	q, err = ParseParameters(`oslc.orderBy=sysml:owner{sysml:owner{-sysml:name}},-sysml:owner{sysml:name,sysml:qualifiedName}`, nil)
	if err != nil {
		t.Fatal(err)
	}
	want = []OrderTerm{{Property: "owner.owner.name", Desc: true}, {Property: "owner.name", Desc: true}, {Property: "owner.qualifiedName", Desc: true}}
	if !slices.Equal(q.OrderBy, want) {
		t.Fatalf("order = %#v, want %#v", q.OrderBy, want)
	}
	_, err = ParseParameters(`oslc.orderBy=-sysml:owner{%2Bsysml:name}`, nil)
	if got := kindOf(t, err, ErrMalformed); !strings.Contains(got.Message, "signed both inside and outside") {
		t.Fatalf("error = %q", got.Message)
	}
	for _, text := range []string{
		`oslc.orderBy=sysml:owner{}`, `oslc.orderBy=sysml:owner{sysml:name`, `oslc.orderBy=sysml:owner{sysml:name}}`,
		`oslc.orderBy=sysml:name,`, `oslc.orderBy=,sysml:name`, `oslc.orderBy=sysml:name sysml:body`,
		`oslc.orderBy=sysml:owner{rdf:type}`, `oslc.orderBy=sysml:owner{*}`,
	} {
		if _, err := ParseParameters(text, nil); err == nil {
			t.Errorf("%q unexpectedly parsed", text)
		} else {
			kindOf(t, err, ErrMalformed)
		}
	}
}

// term ::= identifier_wc space in_op space? in_val: the space after "in" is optional.
func TestParseOSLCInTakesItsListWithoutASpace(t *testing.T) {
	for _, text := range []string{`sysml:name in["a","b"]`, `sysml:name in ["a","b"]`, `sysml:name in[ "a" , "b" ]`} {
		if c := mustParse(t, text).Where.(Comparison); c.Operator != In || len(c.Values) != 2 {
			t.Fatalf("ParseOSLC(%q) = %#v", text, c)
		}
	}
}

// uri_ref_esc escapes '>' and '\' with a backslash.
func TestParseOSLCDecodesURIEscapes(t *testing.T) {
	if c := mustParse(t, `sysml:documentation=<http://a/b\>c\\d>`).Where.(Comparison); c.Values[0] != `http://a/b>c\d` {
		t.Fatalf("value = %q", c.Values[0])
	}
	for _, text := range []string{`sysml:documentation=<http://a\x>`, `sysml:documentation=<http://a\>`, `sysml:documentation=<http://a\`} {
		_, err := ParseOSLC(text, nil)
		kindOf(t, err, ErrMalformed)
	}
}

// An rdf:XMLLiteral MUST be supported, and MAY be read as a plain string (QUERY-38);
// the datatype is a prefixed name, read under the query's bindings.
func TestParseOSLCReadsXMLLiteralAsAPlainString(t *testing.T) {
	for _, text := range []string{`sysml:name="x"^^rdf:XMLLiteral`, `sysml:name="x"^^xsd:string`, `sysml:name="x"`} {
		if c := mustParse(t, text).Where.(Comparison); c.Values[0] != "x" {
			t.Fatalf("ParseOSLC(%q) = %#v", text, c)
		}
	}
	q, err := ParseParameters(`oslc.prefix=r%3D%3Chttp://www.w3.org/1999/02/22-rdf-syntax-ns%23%3E,x%3D%3Chttp://www.w3.org/2001/XMLSchema%23%3E&oslc.where=sysml:name%3D%22x%22%5E%5Er:XMLLiteral+and+sysml:body%3D%22y%22%5E%5Ex:string`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if terms := q.Where.(Conjunction).Terms; terms[0].(Comparison).Values[0] != "x" || terms[1].(Comparison).Values[0] != "y" {
		t.Fatalf("where = %#v", q.Where)
	}
	for _, text := range []string{`sysml:name="x"^^rdf:HTML`, `sysml:name="x"^^xsd:date`} {
		_, err := ParseOSLC(text, nil)
		kindOf(t, err, ErrUnsupportedLiteral)
	}
	_, err = ParseOSLC(`sysml:name="x"^^ex:thing`, nil)
	if got := kindOf(t, err, ErrMalformed); !strings.Contains(got.Message, `"ex" is unbound`) {
		t.Fatalf("error = %q", got.Message)
	}
}

// decimal is the lexical form of xsd:decimal; what strconv.ParseFloat admits
// besides is refused before it reaches the evaluator.
func TestParseOSLCDecimalsAreXSDDecimals(t *testing.T) {
	for _, text := range []string{"42", "-3.5", "+2", ".5", "42."} {
		if c := mustParse(t, `sysx:multiplicityUpper<`+text).Where.(Comparison); c.Values[0] != text {
			t.Fatalf("value = %#v, want %q", c.Values, text)
		}
	}
	for _, text := range []string{"inf", "Infinity", "NaN", "1e3", "0x1p-2"} {
		_, err := ParseOSLC(`sysx:multiplicityUpper<`+text, nil)
		if got := kindOf(t, err, ErrMalformed); !strings.Contains(got.Message, "decimal") {
			t.Fatalf("ParseOSLC(<%s) error = %q, want the decimal form named", text, got.Message)
		}
		_, err = ParseOSLC(`sysml:name=`+text, nil)
		kindOf(t, err, ErrMalformed)
	}
	// The structured query refuses the same operands, and reads a quoted one.
	for _, text := range []string{"inf", "NaN", "0x1p-2"} {
		_, err := Translate(Query{Where: Comparison{Property: "multiplicityUpper", Operator: Less, Values: []string{text}}}, Context{}, "Test")
		kindOf(t, err, ErrUnparsableValue)
	}
	if _, err := Translate(Query{Where: Comparison{Property: "multiplicityUpper", Operator: Less, Values: []string{"1e1"}}}, Context{}, "Test"); err != nil {
		t.Fatalf("1e1: %v", err)
	}
}

func TestParseOSLCRefusesUnquotedNames(t *testing.T) {
	_, err := ParseOSLC(`sysml:qualifiedName=Robot::Platform::battery`, nil)
	if got := kindOf(t, err, ErrMalformed); !strings.Contains(got.Message, "quoted literal") {
		t.Fatalf("error message = %q, want it to name the quoted form", got.Message)
	}
	mustParse(t, `sysml:qualifiedName="Robot::Platform::battery"`)
	_, err = ParseOSLC(`sysml:name=battery`, nil)
	if got := kindOf(t, err, ErrMalformed); !strings.Contains(got.Message, `quoted literal "battery"`) {
		t.Fatalf("error message = %q, want it to name the quoted form", got.Message)
	}
	q, err := ParseParameters(`oslc.prefix=ex%3D%3Chttps://example.org/%3E&oslc.where=sysml:name%3Dex:a::b`, nil)
	if err != nil {
		t.Fatalf("prefixed name with a local \"::\": %v", err)
	}
	if c := q.Where.(Comparison); c.Values[0] != "https://example.org/a::b" {
		t.Fatalf("value = %#v", c.Values)
	}
}

func TestParseOSLCPropertyDiagnosticsFollowTheActiveBindings(t *testing.T) {
	for _, text := range []string{`@type="x"`, `@id="x"`} {
		_, err := ParseOSLC(text, nil)
		if got := kindOf(t, err, ErrMalformed); !strings.Contains(got.Message, "rdf:type") && !strings.Contains(got.Message, "reported for every result") {
			t.Errorf("%q error message = %q, want it to name the OSLC spelling", text, got.Message)
		}
	}
	aliased := `oslc.prefix=s%3D%3Chttps://www.omg.org/spec/SysML%23%3E`
	_, err := ParseParameters(aliased+`&oslc.where=s:nmae%3D%22x%22`, nil)
	if got := kindOf(t, err, ErrUnknownProperty); !strings.Contains(got.Message, "s:name") {
		t.Errorf("error message = %q, want the aliased spelling offered", got.Message)
	}
	_, err = ParseParameters(`oslc.prefix=sysml%3D%3Curn:example:other%23%3E&oslc.where=sysml:name%3D%22x%22`, nil)
	if got := kindOf(t, err, ErrUnknownProperty); strings.Contains(got.Message, "did you mean") || !strings.Contains(got.Message, "have no oslc.prefix binding") {
		t.Errorf("error message = %q, want the SysML namespace reported as unnamed", got.Message)
	}
	_, err = ParseParameters(`oslc.prefix=%21s%3D%3Chttps://www.omg.org/spec/SysML%23%3E&oslc.where=sysml:name%3D%22x%22`, nil)
	if got := kindOf(t, err, ErrMalformed); !strings.Contains(got.Message, "!s") {
		t.Errorf("error = %#v, want ErrMalformed naming the unusable prefix", err)
	}
	names, unbound := PrefixedPropertyNames(map[string]string{"rdf": rdf.RDFNS}, NewTable(nil, nil))
	if len(names) != 1 || names[0] != "rdf:type" || len(unbound[rdf.SysML]) == 0 || len(unbound[rdf.OpenSysML]) != 2 {
		t.Errorf("names = %#v, unbound = %#v", names, unbound)
	}
}

func TestParseParametersPreservesEncodedSemicolonAndDecodesValues(t *testing.T) {
	for text, want := range map[string]string{
		`oslc.where=sysml%3Aname%3D%22a%253Bb%2Bc%22`: "a%3Bb+c",
		`oslc.where=sysml%3Aname%3D%22a%3Bb%2Bc%22`:   "a;b+c",
		`oslc.where=sysml%3Aname%3D%22a+b%22`:         "a b",
	} {
		q, err := ParseParameters(text, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := q.Where.(Comparison).Values[0]; got != want {
			t.Fatalf("%s value = %q, want %q", text, got, want)
		}
	}
	if _, err := ParseParameters(`oslc.where=`+url.QueryEscape(`sysml:name="a"`+"\u00a0and sysml:name=\"b\""), nil); err != nil {
		t.Fatalf("non-ASCII space before a keyword: %v", err)
	}
}

func TestParseOSLCMalformedNeverPanics(t *testing.T) {
	for _, text := range []string{
		``, `sysml:name`, `sysml:name=`, `sysml:name="x`, `sysml:name="x" and`, `sysml:owner{`, `sysml:owner{sysml:name="x"`,
		`rdf:type in [`, `rdf:type in [sysml:PartUsage`, `oslc.select=sysml:owner{`, `oslc.select=sysml:name,`,
		`oslc.orderBy=+`, `oslc.prefix=x`, `sysml:name="x"^^xsd:date`, `sysml:name="x"^^ex:thing`,
	} {
		if _, err := ParseOSLC(text, nil); err == nil {
			t.Errorf("%q unexpectedly parsed", text)
		}
	}
}

func TestTranslateBuildsTheAgreedPlan(t *testing.T) {
	q, err := ParseParameters(`oslc.where=rdf:type%3Dsysml:PartUsage+and+sysml:name+in+%5B%22a%22,%22b%22%5D+and+sysml:owner%7Bsysml:name%3D%22Demo%22%7D&oslc.select=sysml:name,sysml:owner{sysml:name}&oslc.orderBy=%2Bsysml:name,-sysx:multiplicityUpper`, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Translate(q, Context{}, "Test")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(plan.Columns, " ") != "name owner.name" || plan.Objects {
		t.Fatalf("plan = %#v", plan)
	}
	ops := operations(plan.Program.Definitions()[0].Expression())
	// in [a, b] keeps source order: the rows left once each alternative's are
	// taken away are the ones excluded.
	want := "order-by order-by project where-feature except except where-name where-type parameter except where-name where-type parameter where-type parameter where-type parameter"
	if got := strings.Join(ops, " "); got != want {
		t.Fatalf("operations = %s\nwant %s", got, want)
	}
	plan, err = Translate(mustParse(t, `rdf:type=elmt:`+rdf.EncodeElementID("Demo::Vehicle")+` and sysml:name="car"`), Context{}, "Test")
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Objects || strings.Join(operations(plan.Program.Definitions()[0].Expression()), " ") != "project where-name objects" {
		t.Fatalf("instance plan = %#v", operations(plan.Program.Definitions()[0].Expression()))
	}
}

func TestTranslateRefusesBeforeAnyElementIsRead(t *testing.T) {
	tests := map[string]struct {
		q    Query
		kind ErrorKind
	}{
		"unknown property":       {Query{Where: Comparison{Property: "colour", Operator: Equal, Values: []string{"red"}}}, ErrUnknownProperty},
		"unknown selection":      {Query{Select: []Selection{{Property: "effectiveName"}}}, ErrUnknownProperty},
		"unordered property":     {Query{Where: Comparison{Property: "name", Operator: Greater, Values: []string{"1"}}}, ErrUnorderedProperty},
		"unparsable operand":     {Query{Where: Comparison{Property: "multiplicityUpper", Operator: Less, Values: []string{"many"}}}, ErrUnparsableValue},
		"ordered needs one":      {Query{Where: Comparison{Property: "multiplicityUpper", Operator: Less, Values: []string{"1", "2"}}}, ErrMalformed},
		"no operator":            {Query{Where: Comparison{Property: "name", Values: []string{"x"}}}, ErrMalformed},
		"instanceOf off @type":   {Query{Where: Comparison{Property: "name", Operator: InstanceOf, Values: []string{"Usage"}}}, ErrUnorderedProperty},
		"undeclared metaclass":   {Query{Where: TypeTest{Types: []string{"Gizmo"}}}, ErrUnparsableValue},
		"nested instance term":   {Query{Where: Disjunction{Terms: []Constraint{Instances{Elements: []string{"Demo::A"}}, TypeTest{Types: []string{"PartUsage"}}}}}, ErrUnsupportedInstanceTerm},
		"scoped instance term":   {Query{Where: Instances{Elements: []string{"Demo::A"}}, Scope: []string{"Demo"}}, ErrUnsupportedInstanceTerm},
		"scoped scalar":          {Query{Where: Scoped{Property: "name", Inner: Comparison{Property: "name", Operator: Equal, Values: []string{"x"}}}}, ErrUnorderedProperty},
		"non-boolean isAbstract": {Query{Where: Comparison{Property: "isAbstract", Operator: Equal, Values: []string{"maybe"}}}, ErrUnparsableValue},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Translate(tc.q, Context{}, "Test")
			if err == nil {
				t.Fatal("unexpectedly translated")
			}
			kindOf(t, err, tc.kind)
		})
	}
}

// operations lists the plan's operations outermost first, following each
// operation's source (or exclude, then source) down to the root.
func operations(expression queryplan.Expression) []string {
	out := []string{string(expression.Operation())}
	var source *queryplan.Expression
	for _, argument := range expression.Arguments() {
		value := argument.Value
		switch argument.Name {
		case "exclude":
			out = append(out, operations(value)...)
		case "source", "other":
			source = &value
		}
	}
	if source != nil {
		out = append(out, operations(*source)...)
	}
	return out
}

// A row lacking the sort key sorts first ascending and last descending, as the
// lexical evaluator ordered.
func TestTranslateOrdersAMissingKeyAsTheSmallestValue(t *testing.T) {
	plan, err := Translate(mustParse(t, "oslc.orderBy=+sysml:name,-sysml:body"), Context{}, "Test")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for expression := plan.Program.Definitions()[0].Expression(); expression.Operation() == queryplan.OperationOrderBy; {
		var property, missing string
		var source queryplan.Expression
		for _, argument := range expression.Arguments() {
			switch argument.Name {
			case "property":
				_, property = argument.Value.Literal()
			case "missing":
				_, missing = argument.Value.Literal()
			case "source":
				source = argument.Value
			}
		}
		got[property] = missing
		expression = source
	}
	if want := map[string]string{"name": "first", "body": "last"}; !maps.Equal(got, want) {
		t.Fatalf("missing keys sort = %v, want %v", got, want)
	}
}

// A query selecting nothing reports the properties of particular metaclasses too,
// absent on the rows without them; elementId alone waits to be named.
func TestDefaultProjectionReportsEveryPropertyButElementID(t *testing.T) {
	got := DefaultProjection()
	for _, name := range []string{query.PropertyName, query.PropertyBody, query.PropertyLocale, query.PropertyLanguage,
		query.PropertySatisfiedRequirement, query.PropertySatisfyingFeature, query.PropertyPayloadArgument,
		query.PropertySenderArgument, query.PropertyReceiverArgument, query.PropertyMultiplicityUpper} {
		if !slices.Contains(got, name) {
			t.Errorf("DefaultProjection lacks %s: %v", name, got)
		}
	}
	if slices.Contains(got, query.PropertyElementID) {
		t.Errorf("DefaultProjection reads elementId: %v", got)
	}
}
