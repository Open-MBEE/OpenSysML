package query

import (
	"net/url"
	"strings"
	"testing"
)

func TestParseOSLCOperatorsAndValues(t *testing.T) {
	q, err := ParseOSLC(`sysml:name="wheel" and sysml:multiplicityLower>=2 and sysml:type in [sysml:PartUsage, "x"]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Where) != 3 || q.Where[1].Operator != GreaterEqual || q.Where[2].Operator != In {
		t.Fatalf("where = %#v", q.Where)
	}
	if q.Where[0].Values[0] != "wheel" {
		t.Fatalf("string value = %#v", q.Where[0].Values)
	}
	if q.Where[2].Values[0] != "PartUsage" {
		t.Fatalf("prefixed value = %#v", q.Where[2].Values)
	}
}

func TestParseOSLCParametersAndPrefixes(t *testing.T) {
	q, err := ParseParameters(`oslc.prefix=ex%3D%3Curn%3Aexample%3A%3E&oslc.where=sysml%3Aname%3D%22x%22&oslc.select=sysml%3Aname%2Crdf%3Atype&oslc.orderBy=-sysml%3Aname`)
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Select) != 2 || q.Select[0] != PropertyName || q.Select[1] != PropertyType {
		t.Fatalf("select = %#v", q.Select)
	}
	if len(q.OrderBy) != 1 || !q.OrderBy[0].Desc {
		t.Fatalf("order = %#v", q.OrderBy)
	}
	if q.SpellingOf(PropertyName) != "sysml:name" || q.SpellingOf(PropertyType) != "rdf:type" {
		t.Fatalf("spelling = %#v", q.Spelling)
	}
}

func TestParseOSLCKeepsTheSpellingOfASelectedProperty(t *testing.T) {
	// A selected property is reported under the name the request asked for, so
	// an answer can be pasted back into a query.
	q, err := ParseParameters(`oslc.prefix=s%3D%3Chttps://www.omg.org/spec/SysML%23%3E&oslc.select=s%3Aname`)
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Select) != 1 || q.Select[0] != PropertyName {
		t.Fatalf("select = %#v", q.Select)
	}
	if got := q.SpellingOf(PropertyName); got != "s:name" {
		t.Fatalf("spelling of %s = %q, want the aliased name", PropertyName, got)
	}
	// A property a request never named keeps its query property name.
	if got := q.SpellingOf(PropertyOwner); got != PropertyOwner {
		t.Fatalf("spelling of an unasked property = %q", got)
	}
}

func TestParseOSLCRefusesAPropertySelectedTwice(t *testing.T) {
	// One property is reported once, so two spellings of it in one selection
	// cannot both be honored and are refused rather than picked between.
	const bound = `oslc.prefix=s%3D%3Chttps://www.omg.org/spec/SysML%23%3E`
	for _, tt := range []struct{ selection, message string }{
		{`sysml%3Aname%2Cs%3Aname`, `"sysml:name" and "s:name" are the same property`},
		{`sysml%3Aname%2Csysml%3Aname`, `names "sysml:name" twice`},
	} {
		q, err := ParseParameters(bound + `&oslc.select=` + tt.selection)
		if err == nil {
			t.Fatalf("%s unexpectedly parsed as %#v", tt.selection, q)
		}
		got, ok := err.(*Error)
		if !ok || got.Kind != ErrMalformed {
			t.Fatalf("%s error = %#v, want ErrMalformed", tt.selection, err)
		}
		if !strings.Contains(got.Message, tt.message) {
			t.Fatalf("%s error message = %q, want it to contain %q", tt.selection, got.Message, tt.message)
		}
	}
	// Two different properties under two prefixes stay a valid selection.
	q, err := ParseParameters(bound + `&oslc.select=sysml%3Aname%2Cs%3Aowner`)
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Select) != 2 || q.SpellingOf(PropertyName) != "sysml:name" || q.SpellingOf(PropertyOwner) != "s:owner" {
		t.Fatalf("select = %#v, spelling = %#v", q.Select, q.Spelling)
	}
}

func TestParseOSLCRejectsUnsupportedConstructs(t *testing.T) {
	tests := []struct {
		text string
		kind ErrorKind
	}{
		{`sysml:owner{sysml:name="x"}`, ErrUnsupportedScopedTerm},
		{`oslc.select=*`, ErrUnsupportedWildcard},
		{`oslc.orderBy=*`, ErrUnsupportedWildcard},
		{`oslc.where=*=x`, ErrUnsupportedWildcard},
		{`oslc.properties=*`, ErrUnsupportedWildcard},
		{`oslc.searchTerms=x`, ErrUnsupportedSearchTerms},
		{`oslc.searchTerms=`, ErrUnsupportedSearchTerms},
		{`sysml:name="x"@en`, ErrUnsupportedLiteral},
	}
	for _, tt := range tests {
		q, err := ParseOSLC(tt.text)
		if err == nil {
			t.Errorf("%q unexpectedly parsed as %#v", tt.text, q)
			continue
		}
		got, ok := err.(*Error)
		if !ok || got.Kind != tt.kind {
			t.Errorf("%q error = %#v, want kind %d", tt.text, err, tt.kind)
		}
	}
}

func TestParseOSLCAllowsInfinityForOrderedMultiplicity(t *testing.T) {
	for _, text := range []string{
		`sysml:multiplicityUpper>=*`,
		`sysml:multiplicityUpper=*`,
		`sysml:multiplicityUpper in [*, 1]`,
		`sysml:name=*`,
	} {
		q, err := ParseOSLC(text)
		if err != nil {
			t.Fatalf("ParseOSLC(%q): %v", text, err)
		}
		if len(q.Where) != 1 || q.Where[0].Values[0] != "*" {
			t.Fatalf("ParseOSLC(%q) = %#v", text, q.Where)
		}
	}
}

func TestOSLCPropertyMappingCoversQueryableProperties(t *testing.T) {
	reachable := make(map[string]bool)
	for iri, property := range oslcPropertyMappings {
		if !IsProperty(property) {
			t.Errorf("OSLC mapping %q = %q, not in property table", iri, property)
		}
		reachable[property] = true
	}
	for _, property := range PropertyNames() {
		if property != PropertyID && !reachable[property] {
			t.Errorf("property %q has no OSLC mapping", property)
		}
	}
	if property, err := resolveProperty("sysml:qualifiedName", DefaultPrefixes); err != nil || property != PropertyQualifiedName {
		t.Fatalf("sysml:qualifiedName mapping = %q, %v", property, err)
	}
	if !reachable[PropertyQualifiedName] {
		t.Fatal("sysml:qualifiedName does not cover element identity")
	}
	rebound := clonePrefixes(DefaultPrefixes)
	rebound["sysml"] = "urn:custom:"
	if _, err := resolveProperty("sysml:name", rebound); err == nil {
		t.Fatal("rebound sysml prefix unexpectedly selected a built-in property")
	}
}

func TestParseOSLCReducesURIAndPrefixedValuesAlike(t *testing.T) {
	for _, text := range []string{
		`rdf:type=<https://www.omg.org/spec/SysML#PartUsage>`,
		`rdf:type=sysml:PartUsage`,
		`rdf:type="PartUsage"`,
	} {
		q, err := ParseOSLC(text)
		if err != nil {
			t.Fatalf("ParseOSLC(%q): %v", text, err)
		}
		if len(q.Where) != 1 || q.Where[0].Values[0] != "PartUsage" {
			t.Fatalf("ParseOSLC(%q) = %#v, want the value PartUsage", text, q.Where)
		}
	}
	q, err := ParseOSLC(`sysml:name=<urn:example:battery>`)
	if err != nil {
		t.Fatal(err)
	}
	if q.Where[0].Values[0] != "urn:example:battery" {
		t.Fatalf("value = %#v, want a foreign IRI kept whole", q.Where[0].Values)
	}
}

func TestParseOSLCRefusesParametersItDoesNotRead(t *testing.T) {
	tests := []string{
		`oslc.wheree=sysml:name%3D%22battery%22`,
		`oslc.pageSize=10&oslc.where=sysml:name%3D%22battery%22`,
		`oslc.where=sysml:name%3D%22battery%22&oslc.where=sysml:name%3D%22gripper%22`,
		`oslc.where=`,
		`oslc.select=`,
		`oslc.orderBy=`,
		`oslc.prefix=`,
		`oslc.select=+&oslc.where=sysml:name%3D%22battery%22`,
		`oslc.properties=sysml:name`,
	}
	for _, text := range tests {
		q, err := ParseOSLC(text)
		if err == nil {
			t.Errorf("%q unexpectedly parsed as %#v", text, q)
			continue
		}
		if got, ok := err.(*Error); !ok || got.Kind != ErrMalformed {
			t.Errorf("%q error = %#v, want ErrMalformed", text, err)
		}
	}
}

func TestParseOSLCRefusesQualifiedNameValueUnquoted(t *testing.T) {
	q, err := ParseOSLC(`sysml:qualifiedName=Robot::Platform::battery`)
	if err == nil {
		t.Fatalf("unexpectedly parsed as %#v", q)
	}
	got, ok := err.(*Error)
	if !ok || got.Kind != ErrMalformed {
		t.Fatalf("error = %#v, want ErrMalformed", err)
	}
	if !strings.Contains(got.Message, "quoted literal") {
		t.Fatalf("error message = %q, want it to name the quoted form", got.Message)
	}
	if _, err := ParseOSLC(`sysml:qualifiedName="Robot::Platform::battery"`); err != nil {
		t.Fatalf("quoted qualified name: %v", err)
	}
	// A "::" inside the local part of a prefixed name is that name's business.
	q, err = ParseParameters(`oslc.prefix=ex%3D%3Chttps://example.org/%3E&oslc.where=sysml:name%3Dex:a::b`)
	if err != nil {
		t.Fatalf("prefixed name with a local \"::\": %v", err)
	}
	if q.Where[0].Values[0] != "https://example.org/a::b" {
		t.Fatalf("value = %#v", q.Where[0].Values)
	}
}

func TestParseOSLCRefusesBareNameValueWithTheQuotedForm(t *testing.T) {
	// A reported value is a bare name, so its refusal names the quoted form.
	_, err := ParseOSLC(`sysml:name=battery`)
	got, ok := err.(*Error)
	if !ok || got.Kind != ErrMalformed {
		t.Fatalf("error = %#v, want ErrMalformed", err)
	}
	if !strings.Contains(got.Message, `quoted literal "battery"`) {
		t.Fatalf("error message = %q, want it to name the quoted form", got.Message)
	}
	if _, err := ParseOSLC(`sysml:name="battery"`); err != nil {
		t.Fatalf("quoted name: %v", err)
	}
}

func TestParseOSLCPropertyDiagnosticsNameOSLCSpellings(t *testing.T) {
	// The list an unknown property is answered with must be writable as it reads.
	_, err := ParseOSLC(`sysml:id="x"`)
	got, ok := err.(*Error)
	if !ok || got.Kind != ErrUnknownProperty {
		t.Fatalf("error = %#v, want ErrUnknownProperty", err)
	}
	names, unbound := PrefixedPropertyNames(DefaultPrefixes)
	if len(unbound) != 0 {
		t.Errorf("default prefixes leave %v unnamed", unbound)
	}
	for _, name := range names {
		if !strings.Contains(got.Message, name) {
			t.Errorf("error message = %q, want it to name %q", got.Message, name)
		}
		if _, err := ParseOSLC(name + `="x"`); err != nil {
			t.Errorf("listed property %q: %v", name, err)
		}
	}
	for _, name := range []string{PropertyID, PropertyType} {
		if strings.Contains(got.Message, name) {
			t.Errorf("error message = %q, want it to omit the Go API name %q", got.Message, name)
		}
	}

	// "@type" and "@id" name themselves in the Go API; OSLC query text cannot.
	for _, text := range []string{
		`@type=sysml:PartUsage`,
		`@id="x"`,
		`oslc.where=sysml:name%3D%22x%22&oslc.select=@id`,
		`oslc.where=sysml:name%3D%22x%22&oslc.orderBy=@type`,
	} {
		q, err := ParseOSLC(text)
		got, ok := err.(*Error)
		if !ok || got.Kind != ErrMalformed {
			t.Errorf("%q parsed as %#v, error = %#v, want ErrMalformed", text, q, err)
			continue
		}
		if !strings.Contains(got.Message, "rdf:type") && !strings.Contains(got.Message, "reported for every result") {
			t.Errorf("%q error message = %q, want it to name the OSLC spelling", text, got.Message)
		}
	}
}

func TestParseOSLCPropertyDiagnosticsFollowTheActiveBindings(t *testing.T) {
	// An alias bound to the SysML namespace is what the query must be written with.
	const aliased = `oslc.prefix=s%3D%3Chttps://www.omg.org/spec/SysML%23%3E,sysml%3D%3Curn:example:other%23%3E`
	_, err := ParseParameters(aliased + `&oslc.where=s:id%3D%22x%22`)
	got, ok := err.(*Error)
	if !ok || got.Kind != ErrUnknownProperty {
		t.Fatalf("error = %#v, want ErrUnknownProperty", err)
	}
	for _, name := range []string{"s:name", "s:qualifiedName", "rdf:type"} {
		if !strings.Contains(got.Message, name) {
			t.Errorf("error message = %q, want it to name %q", got.Message, name)
		}
		if _, err := ParseParameters(aliased + `&oslc.where=` + url.QueryEscape(name+`="x"`)); err != nil {
			t.Errorf("listed property %q: %v", name, err)
		}
	}
	if strings.Contains(got.Message, "sysml:name") {
		t.Errorf("error message = %q, want it to omit the rebound spelling \"sysml:name\"", got.Message)
	}

	// A namespace no binding names cannot be offered as a prefixed name at all.
	_, err = ParseParameters(`oslc.prefix=sysml%3D%3Curn:example:other%23%3E&oslc.where=sysml:id%3D%22x%22`)
	got, ok = err.(*Error)
	if !ok || got.Kind != ErrUnknownProperty {
		t.Fatalf("error = %#v, want ErrUnknownProperty", err)
	}
	if strings.Contains(got.Message, "sysml:name") || !strings.Contains(got.Message, "which no oslc.prefix binding names") {
		t.Errorf("error message = %q, want it to report the SysML namespace as unnamed", got.Message)
	}
	// A prefix no prefixed name can be written with is refused at its binding,
	// so it can never be the spelling a diagnostic offers.
	_, err = ParseParameters(`oslc.prefix=%21s%3D%3Chttps://www.omg.org/spec/SysML%23%3E&oslc.where=sysml:name%3D%22x%22`)
	got, ok = err.(*Error)
	if !ok || got.Kind != ErrMalformed || !strings.Contains(got.Message, "!s") {
		t.Errorf("error = %#v, want ErrMalformed naming the unusable prefix", err)
	}

	// A prefix of multi-byte letters binds and is written as it reads, so the
	// spellings a diagnostic offers under it are parseable too.
	const unicodePrefix = "sÿsml"
	bound := `oslc.prefix=` + url.QueryEscape(unicodePrefix+"=<"+sysmlNS+">,sysml=<urn:example:other#>")
	_, err = ParseParameters(bound + `&oslc.where=` + url.QueryEscape(unicodePrefix+`:id="x"`))
	got, ok = err.(*Error)
	if !ok || got.Kind != ErrUnknownProperty || !strings.Contains(got.Message, unicodePrefix+":name") {
		t.Fatalf("error = %#v, want ErrUnknownProperty naming %q", err, unicodePrefix+":name")
	}
	for _, parameter := range []string{
		`oslc.where=` + url.QueryEscape(unicodePrefix+`:name="battery"`),
		`oslc.where=` + url.QueryEscape(unicodePrefix+`:name="battery"`) + `&oslc.select=` + url.QueryEscape(unicodePrefix+":name"),
		`oslc.where=` + url.QueryEscape(unicodePrefix+`:name="battery"`) + `&oslc.orderBy=` + url.QueryEscape("-"+unicodePrefix+":name"),
	} {
		if _, err := ParseParameters(bound + "&" + parameter); err != nil {
			t.Errorf("%s: %v", parameter, err)
		}
	}

	names, unbound := PrefixedPropertyNames(map[string]string{"rdf": rdfNS})
	if len(names) != 1 || names[0] != "rdf:type" {
		t.Errorf("names = %#v, want [rdf:type]", names)
	}
	if locals := unbound[sysmlNS]; len(locals) == 0 || locals[0] != "declaredName" {
		t.Errorf("unbound[SysML] = %#v, want the SysML local names", locals)
	}
}

func TestParseParametersPreservesEncodedSemicolonAndDecodesValues(t *testing.T) {
	q, err := ParseParameters(`oslc.where=sysml%3Aname%3D%22a%253Bb%2Bc%22`)
	if err != nil {
		t.Fatal(err)
	}
	if got := q.Where[0].Values[0]; got != "a%3Bb+c" {
		t.Fatalf("value = %q, want one URL-decoding pass", got)
	}
	q, err = ParseParameters(`oslc.where=sysml%3Aname%3D%22a%3Bb%2Bc%22`)
	if err != nil {
		t.Fatal(err)
	}
	if got := q.Where[0].Values[0]; got != "a;b+c" {
		t.Fatalf("value = %q, want semicolon preserved", got)
	}
	q, err = ParseParameters(`oslc.where=sysml%3Aname%3D%22a+b%22`)
	if err != nil {
		t.Fatal(err)
	}
	if got := q.Where[0].Values[0]; got != "a b" {
		t.Fatalf("value = %q, want plus decoded as space", got)
	}
}

func TestParseOSLCReadsKeywordsAcrossNonASCIISpace(t *testing.T) {
	// A space the scanner skips must also end a keyword, whatever its width.
	const nbsp = "\u00a0"
	for _, space := range []string{" ", nbsp} {
		q, err := ParseOSLC(`sysml:name="battery"` + space + `and` + space + `rdf:type=sysml:PartUsage`)
		if err != nil || len(q.Where) != 2 {
			t.Errorf("%q separated conjunction = %#v, error = %v, want two terms", space, q, err)
		}
		q, err = ParseOSLC(`sysml:name` + space + `in` + space + `["battery","gripper"]`)
		if err != nil || len(q.Where) != 1 || len(q.Where[0].Values) != 2 {
			t.Errorf("%q separated in term = %#v, error = %v, want two values", space, q, err)
		}
	}
}

func TestParseOSLCMalformedNeverPanics(t *testing.T) {
	inputs := []string{"", `sysml:name=`, `sysml:name="`, `sysml:name in [`, `sysml:name="x" or sysml:type="y"`, `sysml:name="spare" and`, `oslc.select=sysml:name,`}
	for _, input := range inputs {
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Errorf("ParseOSLC(%q) panicked: %v", input, recovered)
				}
			}()
			_, err := ParseOSLC(input)
			if err == nil {
				t.Errorf("ParseOSLC(%q) unexpectedly succeeded", input)
				return
			}
			if queryErr, ok := err.(*Error); !ok || queryErr.Kind != ErrMalformed {
				t.Errorf("ParseOSLC(%q) error = %#v, want ErrMalformed", input, err)
			}
		}()
	}
}
