package export

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// duplicateSource declares one name twice in a namespace: each is its own
// element, the first keeping the qualified name, the later identified by its
// position, as the language's own naming rule fixes it.
const duplicateSource = `package P {
 private import ScalarValues::*;
 part def A { attribute x : Real; }
 part def A { attribute y : Real; }
}
`

// duplicateGraph converts duplicateSource into a graph under the id form.
func duplicateGraph(t *testing.T, src string, form IDForm) *rdf.Graph {
	t.Helper()
	file := source.New("d.sysml", []byte(src))
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("the fixture does not parse: %v", p.Diagnostics)
	}
	graph, err := ToRDFWith(file, root, form)
	if err != nil {
		t.Fatalf("ToRDFWith: %v", err)
	}
	return graph
}

func withoutIdentitySourceText(graph *rdf.Graph) *rdf.Graph {
	out := rdf.NewGraph()
	for _, triple := range graph.Triples() {
		if triple.Predicate.Value == rdf.OpenSysML+"sourceText" ||
			triple.Predicate.Value == rdf.OpenSysML+"sourceTail" ||
			triple.Predicate.Value == rdf.OpenSysML+"sourceLanguage" {
			continue
		}
		out.AddTriple(triple)
	}
	return out
}

func elementSubjectsByQualifiedName(graph *rdf.Graph) map[string]string {
	subjects := make(map[string]string)
	for _, subject := range graph.Subjects() {
		if qname, ok := graph.Lexical(subject, rdf.SysML+"qualifiedName"); ok {
			subjects[qname] = subject.Value
		}
	}
	return subjects
}

// apiElements decodes an API JSON document into its element objects.
func apiElements(t *testing.T, document []byte) []map[string]any {
	t.Helper()
	var elements []map[string]any
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatalf("the API JSON does not decode: %v\n%s", err, document)
	}
	return elements
}

// apiID reads an {"@id": id} reference value.
func apiID(v any) string {
	ref, _ := v.(map[string]any)
	id, _ := ref["@id"].(string)
	return id
}

// TestDuplicateMemberNamesExportAPI checks the duplicate-name model exports
// both members as elements: two PartDefinitions named A with distinct ids,
// each owned by P through its own membership, x owned by the first and y by
// the second.
func TestDuplicateMemberNamesExportAPI(t *testing.T) {
	document, err := WriteAPIJSON(duplicateGraph(t, duplicateSource, IDQualifiedName))
	if err != nil {
		t.Fatalf("WriteAPIJSON: %v", err)
	}
	elements := apiElements(t, document)

	var defs []map[string]any
	for _, el := range elements {
		if el["@type"] == "PartDefinition" && el["declaredName"] == "A" {
			defs = append(defs, el)
		}
	}
	if len(defs) != 2 {
		t.Fatalf("got %d `part def A` elements, want 2:\n%s", len(defs), document)
	}
	first, second := defs[0], defs[1]
	if first["@id"] == second["@id"] {
		t.Fatalf("the two `part def A` share an @id %v", first["@id"])
	}
	if first["qualifiedName"] != "P::A" || second["qualifiedName"] != "P::@2" {
		t.Fatalf("qualified names are %v and %v, want P::A and P::@2", first["qualifiedName"], second["qualifiedName"])
	}
	for _, def := range defs {
		if apiID(def["owningNamespace"]) != "P" {
			t.Fatalf("%v is not owned by P:\n%s", def["@id"], document)
		}
	}
	// Each is owned through its own OwningMembership.
	var memberships []map[string]any
	for _, el := range elements {
		if el["@type"] == "OwningMembership" && apiID(el["memberElement"]) == first["@id"] || el["@type"] == "OwningMembership" && apiID(el["memberElement"]) == second["@id"] {
			memberships = append(memberships, el)
		}
	}
	if len(memberships) != 2 || memberships[0]["@id"] == memberships[1]["@id"] {
		t.Fatalf("the two `part def A` do not each have their own membership:\n%s", document)
	}
	// x belongs to the first, y to the second.
	ownerOf := func(name string) string {
		for _, el := range elements {
			if el["@type"] == "AttributeUsage" && el["declaredName"] == name {
				return apiID(el["owningNamespace"])
			}
		}
		return ""
	}
	if ownerOf("x") != first["@id"] || ownerOf("y") != second["@id"] {
		t.Fatalf("x is owned by %q and y by %q, want %v and %v", ownerOf("x"), ownerOf("y"), first["@id"], second["@id"])
	}
}

// TestDuplicateMemberNamesExportTurtle checks the same two elements land in
// the Turtle, named P::A and P::@2, both declared as A.
func TestDuplicateMemberNamesExportTurtle(t *testing.T) {
	turtle := string(rdf.WriteTurtle(duplicateGraph(t, duplicateSource, IDQualifiedName)))
	for _, want := range []string{`sysml:qualifiedName "P::A"`, `sysml:qualifiedName "P::@2"`} {
		if !strings.Contains(turtle, want) {
			t.Fatalf("the Turtle lacks %s:\n%s", want, turtle)
		}
	}
	if n := strings.Count(turtle, `sysml:declaredName "A"`); n != 2 {
		t.Fatalf("got %d `declaredName \"A\"`, want 2:\n%s", n, turtle)
	}
}

// TestDuplicateMemberNamesExportUUID checks the uuid id form keeps the two
// members apart too: two PartDefinitions named A, each its own derived uuid.
func TestDuplicateMemberNamesExportUUID(t *testing.T) {
	document, err := WriteAPIJSON(duplicateGraph(t, duplicateSource, IDUUID))
	if err != nil {
		t.Fatalf("WriteAPIJSON: %v", err)
	}
	var defs []map[string]any
	for _, el := range apiElements(t, document) {
		if el["@type"] == "PartDefinition" && el["declaredName"] == "A" {
			defs = append(defs, el)
		}
	}
	if len(defs) != 2 {
		t.Fatalf("got %d `part def A` elements, want 2:\n%s", len(defs), document)
	}
	if defs[0]["@id"] == defs[1]["@id"] {
		t.Fatalf("the two `part def A` share an @id %v", defs[0]["@id"])
	}
	for _, def := range defs {
		if id, _ := def["@id"].(string); !uuidPattern.MatchString(id) {
			t.Fatalf("id %q is not a uuid", id)
		}
	}
}

// TestDuplicateMemberNamesDeclaredID checks a duplicate carrying an explicit
// ElementId keeps the id it declared rather than taking a derived one.
func TestDuplicateMemberNamesDeclaredID(t *testing.T) {
	const src = `package P {
	part def A;
	part def A {
		@IdentityMetadata::ElementId { id = "aaaa0000-0000-5000-8000-0000000000aa"; }
	}
}`
	document, err := WriteAPIJSON(duplicateGraph(t, src, IDUUID))
	if err != nil {
		t.Fatalf("WriteAPIJSON: %v", err)
	}
	var declared []map[string]any
	for _, el := range apiElements(t, document) {
		if el["elementId"] == "aaaa0000-0000-5000-8000-0000000000aa" {
			declared = append(declared, el)
		}
	}
	if len(declared) != 1 || declared[0]["@type"] != "PartDefinition" {
		t.Fatalf("the declared ElementId did not land on one PartDefinition:\n%s", document)
	}
}

// TestDuplicateMemberNamesRoundTrip checks notation → Turtle → notation →
// Turtle is idempotent and both definitions are in the graph the whole way.
func TestDuplicateMemberNamesRoundTrip(t *testing.T) {
	graph := duplicateGraph(t, duplicateSource, IDQualifiedName)
	notation, err := ToSysML(graph)
	if err != nil {
		t.Fatalf("ToSysML: %v", err)
	}
	for _, want := range []string{"part def A {", "attribute x : Real;", "attribute y : Real;"} {
		if !strings.Contains(string(notation), want) {
			t.Fatalf("the decoded notation lacks %q:\n%s", want, notation)
		}
	}
	file := source.New("back.sysml", notation)
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("the decoded notation does not parse: %v\n%s", p.Diagnostics, notation)
	}
	again, err := ToRDF(file, root)
	if err != nil {
		t.Fatalf("ToRDF of the decoded notation: %v", err)
	}
	if !bytes.Equal(rdf.WriteTurtle(graph), rdf.WriteTurtle(again)) {
		t.Fatalf("the round trip is not idempotent:\n--- first ---\n%s\n--- second ---\n%s", rdf.WriteTurtle(graph), rdf.WriteTurtle(again))
	}
}

// TestDuplicateMemberNamesToolkit checks a graph carrying no qualified names —
// the toolkit's interchange form — decodes duplicate and positional-looking
// names as distinct members. The order an owner lists its ownedMembership is
// the order both the names and the notation take, whatever order the elements
// themselves appear in: a comes first in the list but later in the document,
// and still keeps P::A and prints first.
func TestDuplicateMemberNamesToolkit(t *testing.T) {
	doc := `[
		{"@type": "Package", "@id": "P", "declaredName": "P", "isImpliedIncluded": false,
		 "ownedMembership": [{"@id": "m1"}, {"@id": "m2"}, {"@id": "m3"}]},
		{"@type": "OwningMembership", "@id": "m1", "isImpliedIncluded": false,
		 "memberElement": {"@id": "a"}, "membershipOwningNamespace": {"@id": "P"}},
		{"@type": "OwningMembership", "@id": "m2", "isImpliedIncluded": false,
		 "memberElement": {"@id": "b"}, "membershipOwningNamespace": {"@id": "P"}},
		{"@type": "OwningMembership", "@id": "m3", "isImpliedIncluded": false,
		 "memberElement": {"@id": "c"}, "membershipOwningNamespace": {"@id": "P"}},
		{"@type": "PartDefinition", "@id": "c", "declaredName": "A", "isImpliedIncluded": false,
		 "ownedMembership": [{"@id": "my"}]},
		{"@type": "FeatureMembership", "@id": "my", "isImpliedIncluded": false,
		 "memberElement": {"@id": "y"}, "membershipOwningNamespace": {"@id": "c"}},
		{"@type": "AttributeUsage", "@id": "y", "declaredName": "y", "isImpliedIncluded": false},
		{"@type": "PartDefinition", "@id": "b", "declaredName": "@2", "isImpliedIncluded": false},
		{"@type": "PartDefinition", "@id": "a", "declaredName": "A", "isImpliedIncluded": false,
		 "ownedMembership": [{"@id": "mx"}]},
		{"@type": "FeatureMembership", "@id": "mx", "isImpliedIncluded": false,
		 "memberElement": {"@id": "x"}, "membershipOwningNamespace": {"@id": "a"}},
		{"@type": "AttributeUsage", "@id": "x", "declaredName": "x", "isImpliedIncluded": false}
	]`
	graph, err := ReadAPIJSON([]byte(doc))
	if err != nil {
		t.Fatalf("ReadAPIJSON: %v", err)
	}
	metaclasses, err := checkTypes(graph)
	if err != nil {
		t.Fatalf("checkTypes: %v", err)
	}
	normative, err := deriveNormativeGraph(graph, metaclasses)
	if err != nil {
		t.Fatalf("deriveNormativeGraph: %v", err)
	}
	qualified := map[string]string{}
	for _, subject := range normative.Subjects() {
		if name, ok := normative.Lexical(subject, rdf.SysML+pDeclaredName); ok {
			qualified[name], _ = normative.Lexical(subject, rdf.SysML+pQualifiedName)
		}
	}
	for name, want := range map[string]string{
		"x":  "P::A::x",
		"y":  "P::@2::y",
		"@2": "P::'@2'",
	} {
		if qualified[name] != want {
			t.Fatalf("%s has qualified name %q, want %q", name, qualified[name], want)
		}
	}
	out := decodeAPIJSON(t, []byte(doc))
	if n := strings.Count(string(out), "part def A"); n != 2 {
		t.Fatalf("got %d `part def A`, want 2:\n%s", n, out)
	}
	if !strings.Contains(string(out), "part def '@2'") {
		t.Fatalf("the positional-looking name was not quoted:\n%s", out)
	}
	xi := strings.Index(string(out), "x")
	yi := strings.Index(string(out), "y")
	if xi < 0 || yi < 0 || xi > yi {
		t.Fatalf("the A owning x does not precede the A owning y:\n%s", out)
	}
}

// TestDuplicateRootNamesToolkit checks duplicate root names receive distinct
// positional identities, matching the notation export's root ordering.
func TestDuplicateRootNamesToolkit(t *testing.T) {
	doc := `[
		{"@type": "Package", "@id": "p1", "declaredName": "P", "isImpliedIncluded": false,
		 "ownedMembership": [{"@id": "m1"}]},
		{"@type": "Package", "@id": "q", "declaredName": "Q", "isImpliedIncluded": false},
		{"@type": "Package", "@id": "p2", "declaredName": "P", "isImpliedIncluded": false,
		 "ownedMembership": [{"@id": "m2"}]},
		{"@type": "OwningMembership", "@id": "m1", "isImpliedIncluded": false,
		 "memberElement": {"@id": "x1"}, "membershipOwningNamespace": {"@id": "p1"}},
		{"@type": "OwningMembership", "@id": "m2", "isImpliedIncluded": false,
		 "memberElement": {"@id": "x2"}, "membershipOwningNamespace": {"@id": "p2"}},
		{"@type": "PartDefinition", "@id": "x1", "declaredName": "X", "isImpliedIncluded": false},
		{"@type": "PartDefinition", "@id": "x2", "declaredName": "X", "isImpliedIncluded": false}
	]`
	graph, err := ReadAPIJSON([]byte(doc))
	if err != nil {
		t.Fatalf("ReadAPIJSON: %v", err)
	}
	metaclasses, err := checkTypes(graph)
	if err != nil {
		t.Fatalf("checkTypes: %v", err)
	}
	normative, err := deriveNormativeGraph(graph, metaclasses)
	if err != nil {
		t.Fatalf("deriveNormativeGraph: %v", err)
	}
	for id, want := range map[string]string{
		"p1": "P",
		"x1": "P::X",
		"q":  "Q",
		"p2": "@2",
		"x2": "@2::X",
	} {
		subject := rdf.IRI(rdf.Element + id)
		got, ok := normative.Lexical(subject, rdf.SysML+pQualifiedName)
		if !ok || got != want {
			t.Errorf("%s has qualified name %q, want %q", id, got, want)
		}
	}

	file := source.New("duplicate-root.sysml", []byte(
		"package P { part def X; }\npackage Q;\npackage P { part def X; }\n",
	))
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("the notation fixture does not parse: %v", p.Diagnostics)
	}
	notation, err := ToRDF(file, root)
	if err != nil {
		t.Fatalf("ToRDF: %v", err)
	}
	wantNames := []string{"P", "P::X", "Q", "@2", "@2::X"}
	for _, tc := range []struct {
		name  string
		graph *rdf.Graph
	}{
		{name: "toolkit", graph: normative},
		{name: "notation", graph: notation},
	} {
		got := elementSubjectsByQualifiedName(tc.graph)
		if len(got) != len(wantNames) {
			t.Errorf("%s graph has qualified names %v, want %v", tc.name, got, wantNames)
		}
		for _, want := range wantNames {
			if _, ok := got[want]; !ok {
				t.Errorf("%s graph is missing qualified name %q", tc.name, want)
			}
		}
	}

	decoded := decodeAPIJSON(t, []byte(doc))
	if n := strings.Count(string(decoded), "package P"); n != 2 {
		t.Errorf("decoded notation has %d `package P` declarations, want 2:\n%s", n, decoded)
	}
	if n := strings.Count(string(decoded), "part def X"); n != 2 {
		t.Errorf("decoded notation has %d `part def X` declarations, want 2:\n%s", n, decoded)
	}
}

func TestToolkitQuotedIdentityNamesEscape(t *testing.T) {
	for _, test := range []struct {
		name         string
		definition   string
		child        string
		notationName string
	}{
		{
			name:         `a'::b`,
			definition:   `P::'a\'::b'`,
			child:        `P::'a\'::b'::x`,
			notationName: `part def 'a\'::b'`,
		},
		{
			name:       `a::b\`,
			definition: `P::'a::b\\'`,
			child:      `P::'a::b\\'::x`,
		},
	} {
		declaredName, err := json.Marshal(test.name)
		if err != nil {
			t.Fatalf("marshal declared name: %v", err)
		}
		doc := []byte(`[
			{"@type": "Package", "@id": "P", "declaredName": "P", "isImpliedIncluded": false,
			 "ownedMembership": [{"@id": "m1"}, {"@id": "m2"}]},
			{"@type": "OwningMembership", "@id": "m1", "isImpliedIncluded": false,
			 "memberElement": {"@id": "definition"}, "membershipOwningNamespace": {"@id": "P"}},
			{"@type": "OwningMembership", "@id": "m2", "isImpliedIncluded": false,
			 "memberElement": {"@id": "sibling"}, "membershipOwningNamespace": {"@id": "P"}},
			{"@type": "PartDefinition", "@id": "definition", "declaredName": ` + string(declaredName) + `, "isImpliedIncluded": false,
			 "ownedMembership": [{"@id": "featureMembership"}]},
			{"@type": "FeatureMembership", "@id": "featureMembership", "isImpliedIncluded": false,
			 "memberElement": {"@id": "x"}, "membershipOwningNamespace": {"@id": "definition"}},
			{"@type": "PartUsage", "@id": "x", "declaredName": "x", "isImpliedIncluded": false},
			{"@type": "PartDefinition", "@id": "sibling", "declaredName": "a", "isImpliedIncluded": false}
		]`)
		graph, err := ReadAPIJSON(doc)
		if err != nil {
			t.Fatalf("ReadAPIJSON for %q: %v", test.name, err)
		}
		metaclasses, err := checkTypes(graph)
		if err != nil {
			t.Fatalf("checkTypes for %q: %v", test.name, err)
		}
		normative, err := deriveNormativeGraph(graph, metaclasses)
		if err != nil {
			t.Fatalf("deriveNormativeGraph for %q: %v", test.name, err)
		}
		qualified := elementSubjectsByQualifiedName(normative)
		for _, name := range []string{test.definition, test.child, "P::a"} {
			if qualified[name] == "" {
				t.Errorf("%q: no subject has qualified name %q", test.name, name)
			}
		}
		if got := plainQualifiedName(test.child); got != test.child {
			t.Errorf("plainQualifiedName(%q) = %q", test.child, got)
		}
		if test.notationName != "" {
			notation := decodeAPIJSON(t, doc)
			if !strings.Contains(string(notation), test.notationName) || !strings.Contains(string(notation), "part x;") {
				t.Errorf("notation for %q lacks the escaped definition or child:\n%s", test.name, notation)
			}
		}
	}
}

func TestToolkitRootIdentityNamesQuoteSegments(t *testing.T) {
	doc := `[
		{"@type": "PartDefinition", "@id": "rootAB", "declaredName": "A::B", "isImpliedIncluded": false,
		 "ownedMembership": [{"@id": "membershipAB"}]},
		{"@type": "OwningMembership", "@id": "membershipAB", "isImpliedIncluded": false,
		 "memberElement": {"@id": "memberAB"}, "membershipOwningNamespace": {"@id": "rootAB"}},
		{"@type": "PartDefinition", "@id": "memberAB", "declaredName": "x", "isImpliedIncluded": false},
		{"@type": "PartDefinition", "@id": "rootPosition", "declaredName": "@2", "isImpliedIncluded": false,
		 "ownedMembership": [{"@id": "membershipPosition"}]},
		{"@type": "OwningMembership", "@id": "membershipPosition", "isImpliedIncluded": false,
		 "memberElement": {"@id": "memberPosition"}, "membershipOwningNamespace": {"@id": "rootPosition"}},
		{"@type": "PartDefinition", "@id": "memberPosition", "declaredName": "x", "isImpliedIncluded": false},
		{"@type": "PartDefinition", "@id": "Y", "declaredName": "Y", "isImpliedIncluded": false,
		 "ownedSpecialization": [{"@id": "specializationAB"}, {"@id": "specializationPosition"}]},
		{"@type": "Subclassification", "@id": "specializationAB", "isImpliedIncluded": false,
		 "general": {"@id": "memberAB"}, "specific": {"@id": "Y"}, "owningRelatedElement": {"@id": "Y"}},
		{"@type": "Subclassification", "@id": "specializationPosition", "isImpliedIncluded": false,
		 "general": {"@id": "memberPosition"}, "specific": {"@id": "Y"}, "owningRelatedElement": {"@id": "Y"}}
	]`
	graph, err := ReadAPIJSON([]byte(doc))
	if err != nil {
		t.Fatalf("ReadAPIJSON: %v", err)
	}
	metaclasses, err := checkTypes(graph)
	if err != nil {
		t.Fatalf("checkTypes: %v", err)
	}
	normative, err := deriveNormativeGraph(graph, metaclasses)
	if err != nil {
		t.Fatalf("deriveNormativeGraph: %v", err)
	}
	qualified := elementSubjectsByQualifiedName(normative)
	for _, name := range []string{
		`'A::B'`,
		`'A::B'::x`,
		`'@2'`,
		`'@2'::x`,
	} {
		if qualified[name] == "" {
			t.Errorf("no subject has qualified name %q", name)
		}
	}
	notation := decodeAPIJSON(t, []byte(doc))
	y := strings.Index(string(notation), "part def Y")
	if y < 0 {
		t.Fatalf("notation has no sibling part def Y:\n%s", notation)
	}
	for _, reference := range []string{`'A::B'::x`, `'@2'::x`} {
		if !strings.Contains(string(notation)[y:], reference) {
			t.Errorf("Y's specialization does not print %q:\n%s", reference, notation)
		}
	}
}

func TestToolkitIdentityNameBeginningWithQuoteIsDistinct(t *testing.T) {
	doc := `[
		{"@type": "Package", "@id": "P", "declaredName": "P", "isImpliedIncluded": false,
		 "ownedMembership": [{"@id": "membershipQuoted"}, {"@id": "membershipPosition"}, {"@id": "membershipY"}]},
		{"@type": "OwningMembership", "@id": "membershipQuoted", "isImpliedIncluded": false,
		 "memberElement": {"@id": "quoted"}, "membershipOwningNamespace": {"@id": "P"}},
		{"@type": "PartDefinition", "@id": "quoted", "declaredName": "'@2'", "isImpliedIncluded": false},
		{"@type": "OwningMembership", "@id": "membershipPosition", "isImpliedIncluded": false,
		 "memberElement": {"@id": "position"}, "membershipOwningNamespace": {"@id": "P"}},
		{"@type": "PartDefinition", "@id": "position", "declaredName": "@2", "isImpliedIncluded": false},
		{"@type": "OwningMembership", "@id": "membershipY", "isImpliedIncluded": false,
		 "memberElement": {"@id": "Y"}, "membershipOwningNamespace": {"@id": "P"}},
		{"@type": "PartDefinition", "@id": "Y", "declaredName": "Y", "isImpliedIncluded": false,
		 "ownedSpecialization": [{"@id": "specializationQuoted"}, {"@id": "specializationPosition"}]},
		{"@type": "Subclassification", "@id": "specializationQuoted", "isImpliedIncluded": false,
		 "general": {"@id": "quoted"}, "specific": {"@id": "Y"}, "owningRelatedElement": {"@id": "Y"}},
		{"@type": "Subclassification", "@id": "specializationPosition", "isImpliedIncluded": false,
		 "general": {"@id": "position"}, "specific": {"@id": "Y"}, "owningRelatedElement": {"@id": "Y"}}
	]`
	graph, err := ReadAPIJSON([]byte(doc))
	if err != nil {
		t.Fatalf("ReadAPIJSON: %v", err)
	}
	metaclasses, err := checkTypes(graph)
	if err != nil {
		t.Fatalf("checkTypes: %v", err)
	}
	normative, err := deriveNormativeGraph(graph, metaclasses)
	if err != nil {
		t.Fatalf("deriveNormativeGraph: %v", err)
	}
	quotedName := `P::'\'@2\''`
	positionName := `P::'@2'`
	qualified := elementSubjectsByQualifiedName(normative)
	quoted, position := qualified[quotedName], qualified[positionName]
	if quoted == "" || position == "" {
		t.Fatalf("missing distinct qualified names %q and %q: %v", quotedName, positionName, qualified)
	}
	if quoted == position {
		t.Fatalf("qualified names %q and %q share subject %q", quotedName, positionName, quoted)
	}

	notation := decodeAPIJSON(t, []byte(doc))
	for _, declaration := range []string{`part def '\'@2\''`, `part def '@2'`} {
		if !strings.Contains(string(notation), declaration) {
			t.Errorf("read-back notation lacks declaration %q:\n%s", declaration, notation)
		}
	}
	reconverted := duplicateGraph(t, string(notation), IDQualifiedName)
	parserQuotedName := `P::\'@2\'`
	parserPositionName := `P::'@2'`
	reconvertedNames := elementSubjectsByQualifiedName(reconverted)
	for _, name := range []string{parserQuotedName, parserPositionName} {
		if reconvertedNames[name] == "" {
			t.Errorf("reconverted notation has no element named %q: %v\n%s", name, reconvertedNames, notation)
		}
	}
	if reconvertedNames[parserQuotedName] == reconvertedNames[parserPositionName] {
		t.Errorf("reconverted names %q and %q share a subject", parserQuotedName, parserPositionName)
	}
	y := rdf.ElementIRI("P::Y")
	targets := reconverted.Objects(y, rdf.SysML+"specializes")
	wantedTargets := map[string]bool{
		rdf.ElementIRI(parserQuotedName).Value:   true,
		rdf.ElementIRI(parserPositionName).Value: true,
	}
	for _, target := range targets {
		if !target.IsIRI() || !wantedTargets[target.Value] {
			t.Errorf("Y specializes unexpected target %v:\n%s", target, notation)
		}
		delete(wantedTargets, target.Value)
	}
	if len(targets) != 2 || len(wantedTargets) != 0 {
		t.Errorf("Y's specialization targets are %v, want %v:\n%s", targets, []string{
			rdf.ElementIRI(parserQuotedName).Value,
			rdf.ElementIRI(parserPositionName).Value,
		}, notation)
	}
}

// TestDuplicateMemberNamesPositionalCollision keeps a named positional-looking
// member distinct from a later duplicate identified by its position.
func TestDuplicateMemberNamesPositionalCollision(t *testing.T) {
	const src = `package Demo {
	part def Dup;
	part def '@2';
	part def Dup;
}`
	want := []string{"Demo::Dup", "Demo::'@2'", "Demo::@2"}
	for _, form := range []IDForm{IDQualifiedName, IDUUID} {
		graph := duplicateGraph(t, src, form)
		subjects := elementSubjectsByQualifiedName(graph)
		ids := make(map[string]bool)
		for _, qname := range want {
			id, ok := subjects[qname]
			if !ok {
				t.Errorf("%v graph lacks qualified name %q: %v", form, qname, subjects)
				continue
			}
			if ids[id] {
				t.Errorf("%v graph reuses element IRI %q", form, id)
			}
			ids[id] = true
			if form == IDQualifiedName && id != rdf.ElementIRI(qname).Value {
				t.Errorf("%s has IRI %q, want %q", qname, id, rdf.ElementIRI(qname).Value)
			}
		}

		document, err := WriteAPIJSON(graph)
		if err != nil {
			t.Fatalf("WriteAPIJSON: %v", err)
		}
		apiIDs := make(map[string]bool)
		for _, element := range apiElements(t, document) {
			qname, _ := element["qualifiedName"].(string)
			switch qname {
			case "Demo::Dup", "Demo::'@2'", "Demo::@2":
				id, _ := element["@id"].(string)
				if apiIDs[id] {
					t.Errorf("API JSON reuses @id %q", id)
				}
				apiIDs[id] = true
			}
		}
		if len(apiIDs) != 3 {
			t.Errorf("API JSON contains %d distinct collision-member IDs, want 3:\n%s", len(apiIDs), document)
		}

		stripped, err := ToSysML(withoutIdentitySourceText(graph))
		if err != nil {
			t.Fatalf("ToSysML without source text: %v", err)
		}
		if strings.Count(string(stripped), "part def Dup") != 2 ||
			!strings.Contains(string(stripped), "part def '@2'") {
			t.Errorf("stripped round trip lost a declaration:\n%s", stripped)
		}
	}
}

func TestPositionalLookingNameAndUnnamedMemberRoundTrip(t *testing.T) {
	const src = `package P {
	part def '@1';
	part def;
	part def X :> '@1';
}`
	graph := duplicateGraph(t, src, IDQualifiedName)
	if got := elementSubjectsByQualifiedName(graph)["P::'@1'"]; got == "" {
		t.Fatal("the named positional-looking member lacks P::'@1'")
	}
	if got := elementSubjectsByQualifiedName(graph)["P::@1"]; got == "" {
		t.Fatal("the unnamed member lacks P::@1")
	}

	back, err := ToSysML(withoutIdentitySourceText(graph))
	if err != nil {
		t.Fatalf("ToSysML without source text: %v", err)
	}
	if !strings.Contains(string(back), "part def X specializes '@1'") {
		t.Fatalf("the reference did not come back as '@1':\n%s", back)
	}
	reexported := duplicateGraph(t, string(back), IDQualifiedName)
	if !reexported.Has(rdf.Triple{
		Subject:   rdf.ElementIRI("P::X"),
		Predicate: rdf.IRI(rdf.SysML + "specializes"),
		Object:    rdf.ElementIRI("P::'@1'"),
	}) {
		t.Fatal("the round-tripped reference no longer targets P::'@1'")
	}
}

func TestQualifiedNameSeparatorInNameRoundTrip(t *testing.T) {
	const src = `package 'A::B' {
	part def X;
	part def Y :> 'A::B'::X;
}
package A {
	package B {
		part def Y;
	}
}`
	graph := duplicateGraph(t, src, IDQualifiedName)
	want := []string{"'A::B'", "'A::B'::X", "'A::B'::Y", "A", "A::B", "A::B::Y"}
	subjects := elementSubjectsByQualifiedName(graph)
	for _, qname := range want {
		if subjects[qname] == "" {
			t.Errorf("graph lacks qualified name %q: %v", qname, subjects)
		}
	}

	back, err := ToSysML(withoutIdentitySourceText(graph))
	if err != nil {
		t.Fatalf("ToSysML without source text: %v", err)
	}
	reexported := duplicateGraph(t, string(back), IDQualifiedName)
	target, ok := reexported.Object(rdf.ElementIRI("'A::B'::Y"), rdf.SysML+"specializes")
	if !ok || target.Value != rdf.ElementIRI("'A::B'::X").Value {
		t.Errorf("round-tripped reference targets %v, want %q", target, rdf.ElementIRI("'A::B'::X").Value)
	}
}
