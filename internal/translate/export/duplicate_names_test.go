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
// the toolkit's interchange form — decodes two same-named members of one
// namespace as two members, the later addressed by position.
func TestDuplicateMemberNamesToolkit(t *testing.T) {
	doc := `[
		{"@type": "Package", "@id": "P", "declaredName": "P",
		 "ownedMembership": [{"@id": "m1"}, {"@id": "m2"}]},
		{"@type": "OwningMembership", "@id": "m1",
		 "memberElement": {"@id": "a"}, "membershipOwningNamespace": {"@id": "P"}},
		{"@type": "OwningMembership", "@id": "m2",
		 "memberElement": {"@id": "b"}, "membershipOwningNamespace": {"@id": "P"}},
		{"@type": "PartDefinition", "@id": "a", "declaredName": "A"},
		{"@type": "PartDefinition", "@id": "b", "declaredName": "A"}
	]`
	out := decodeAPIJSON(t, []byte(doc))
	if n := strings.Count(string(out), "part def A"); n != 2 {
		t.Fatalf("got %d `part def A`, want 2:\n%s", n, out)
	}
}

// TestDuplicateMemberNamesPositionalCollision refuses a model where the
// positional name a duplicate takes is a sibling's declared name.
func TestDuplicateMemberNamesPositionalCollision(t *testing.T) {
	const src = `package Demo {
	part def Dup;
	part def '@2';
	part def Dup;
}`
	file := source.New("d.sysml", []byte(src))
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("the fixture does not parse: %v", p.Diagnostics)
	}
	_, err := ToRDF(file, root)
	if err == nil {
		t.Fatal("a duplicate whose positional name is a sibling's declared name converted")
	}
	if !strings.Contains(err.Error(), "identified by its position as Demo::@2, which a sibling member is named") {
		t.Fatalf("error %q does not name the positional collision", err)
	}
}
