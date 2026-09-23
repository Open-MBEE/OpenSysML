package export

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

func interchangeFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "tests", "export", "testdata", "interchange", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func decodeAPIJSON(t *testing.T, data []byte) []byte {
	t.Helper()
	graph, err := ReadAPIJSON(data)
	if err != nil {
		t.Fatalf("ReadAPIJSON: %v", err)
	}
	out, err := ToSysML(graph)
	if err != nil {
		t.Fatalf("ToSysML: %v", err)
	}
	return out
}

// countTable tallies element objects by @type.
func countTable(t *testing.T, data []byte) map[string]int {
	t.Helper()
	var objects []map[string]any
	if err := json.Unmarshal(data, &objects); err != nil {
		t.Fatalf("the fixture is not a JSON array: %v", err)
	}
	table := map[string]int{}
	for _, object := range objects {
		if typ, ok := object["@type"].(string); ok {
			table[typ]++
		}
	}
	return table
}

// TestToolkitAPIJSONDecodes reads the reference toolkit's compact and full
// interchange of one model back to notation.
func TestToolkitAPIJSONDecodes(t *testing.T) {
	compact := decodeAPIJSON(t, interchangeFixture(t, "p10.toolkit.compact.json"))
	full := decodeAPIJSON(t, interchangeFixture(t, "p10.toolkit.full.json"))
	if !bytes.Equal(compact, full) {
		t.Fatalf("the compact and full forms decode differently:\n--- compact ---\n%s\n--- full ---\n%s", compact, full)
	}
	file := source.New("decoded.sysml", compact)
	p := parser.New(file)
	p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("the decoded notation does not parse: %v", p.Diagnostics)
	}
}

// TestToolkitAPIGoldenNotation pins the notation the toolkit's interchange
// decodes to.
func TestToolkitAPIGoldenNotation(t *testing.T) {
	got := decodeAPIJSON(t, interchangeFixture(t, "p10.toolkit.compact.json"))
	want := interchangeFixture(t, "p10.decoded.golden.sysml")
	if !bytes.Equal(got, want) {
		t.Fatalf("the decoded notation changed:\n--- want ---\n%s\n--- got ---\n%s", want, got)
	}
}

// TestToolkitRootTransparent checks the toolkit's root Namespace wrapper does
// not decode as a member of its own.
func TestToolkitRootTransparent(t *testing.T) {
	out := decodeAPIJSON(t, interchangeFixture(t, "p10.toolkit.compact.json"))
	text := string(out)
	if strings.Count(text, "package ") != 1 || !strings.HasPrefix(text, "package P10") {
		t.Fatalf("the root namespace wrote a package of its own:\n%s", text)
	}
}

// TestToolkitLiteralReferenceEnds checks a {"@ref": "name"} object end spells
// the reference by name, as the kg unit end does.
func TestToolkitLiteralReferenceEnds(t *testing.T) {
	out := decodeAPIJSON(t, interchangeFixture(t, "p10.toolkit.compact.json"))
	if !strings.Contains(string(out), "[kg]") {
		t.Fatalf("the literal reference end did not decode as a name:\n%s", out)
	}
}

// TestToolkitReExportTable decodes the toolkit's compact interchange and
// re-exports it, checking the element table matches but for the transparent
// root namespace and its owning membership.
func TestToolkitReExportTable(t *testing.T) {
	notation := decodeAPIJSON(t, interchangeFixture(t, "p10.toolkit.compact.json"))
	file := source.New("p10.sysml", notation)
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("the decoded notation does not parse: %v", p.Diagnostics)
	}
	graph, err := ToRDF(file, root)
	if err != nil {
		t.Fatalf("ToRDF: %v", err)
	}
	reexported, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatalf("WriteAPIJSON: %v", err)
	}
	got := countTable(t, reexported)
	want := countTable(t, interchangeFixture(t, "p10.toolkit.compact.json"))
	want["Namespace"]--
	want["OwningMembership"]--
	for typ, n := range want {
		if got[typ] != n {
			t.Errorf("@type %s: want %d, got %d", typ, n, got[typ])
		}
	}
	for typ, n := range got {
		if want[typ] == 0 {
			t.Errorf("@type %s: got %d the compact interchange does not state", typ, n)
		}
	}
}

// TestAPIJSONDisagreementRefuses checks a member spelled two ways that disagree
// is refused rather than silently dropped.
func TestAPIJSONDisagreementRefuses(t *testing.T) {
	doc := `[
		{"@type": "Package", "@id": "P", "qualifiedName": "P", "ownedMembership": [{"@id": "P__x_om"}]},
		{"@type": "PartUsage", "@id": "P__x", "qualifiedName": "P::x"},
		{"@type": "PartUsage", "@id": "P__y", "qualifiedName": "P::y"},
		{"@type": "OwningMembership", "@id": "P__x_om", "memberElement": {"@id": "P__x"}, "ownedRelatedElement": {"@id": "P__y"}, "owningRelatedElement": {"@id": "P"}}
	]`
	graph, err := ReadAPIJSON([]byte(doc))
	if err != nil {
		t.Fatalf("ReadAPIJSON: %v", err)
	}
	if _, err := ToSysML(graph); err == nil {
		t.Fatal("a membership whose member ends disagree decoded")
	}
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// TestUUIDIDs checks -id uuid mints name-based uuids for the derived ids, that
// minting is deterministic, and that the default spelling is unchanged.
func TestUUIDIDs(t *testing.T) {
	src := `package P {
	part def Wheel;
	part car : Vehicle { part wheels : Wheel[4]; }
	part def Vehicle { attribute mass; }
	requirement def LightVehicle { subject v : Vehicle; }
}`
	uuids := func(form IDForm) []string {
		file := source.New("p.sysml", []byte(src))
		p := parser.New(file)
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Fatalf("the fixture does not parse: %v", p.Diagnostics)
		}
		graph, err := ToRDFWith(file, root, form)
		if err != nil {
			t.Fatalf("ToRDFWith: %v", err)
		}
		ids := []string{}
		for _, subject := range graph.Subjects() {
			if local := rdf.LocalName(subject.Value); strings.HasPrefix(subject.Value, rdf.Element) {
				ids = append(ids, local)
			}
		}
		return ids
	}
	first := uuids(IDUUID)
	for _, id := range first {
		if !uuidPattern.MatchString(id) {
			t.Errorf("id %q is not a uuid", id)
		}
	}
	second := uuids(IDUUID)
	if len(first) != len(second) {
		t.Fatalf("uuid minting is not deterministic: %d then %d ids", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("uuid minting is not deterministic: %q then %q", first[i], second[i])
		}
	}
	for _, id := range uuids(IDQualifiedName) {
		if uuidPattern.MatchString(id) {
			t.Errorf("the default form minted a uuid %q", id)
		}
	}
}

// TestUUIDRoundTrip checks the uuid form's document imports back to the same
// notation the qualified form's does.
func TestUUIDRoundTrip(t *testing.T) {
	data := interchangeFixture(t, "p10_interchange.sysml")
	decode := func(form IDForm) []byte {
		file := source.New("p10.sysml", data)
		p := parser.New(file)
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Fatalf("the fixture does not parse: %v", p.Diagnostics)
		}
		graph, err := ToRDFWith(file, root, form)
		if err != nil {
			t.Fatalf("ToRDFWith: %v", err)
		}
		document, err := WriteAPIJSON(graph)
		if err != nil {
			t.Fatalf("WriteAPIJSON: %v", err)
		}
		return decodeAPIJSON(t, document)
	}
	qualified, uuid := decode(IDQualifiedName), decode(IDUUID)
	// The id form changes element identity alone, so both forms decode to the
	// same notation: a mode-derived id is implied again, not re-declared.
	if !bytes.Equal(uuid, qualified) {
		t.Fatalf("the uuid form decoded differently than the qualified form:\n%s", uuid)
	}
}

// TestUUIDDeclaredElementID checks an ElementId the notation declared survives
// a uuid-mode hop: derived ids are implied again, a declared one is not.
func TestUUIDDeclaredElementID(t *testing.T) {
	data := []byte(`package P {
    part def A {
        @IdentityMetadata::ElementId { id = "aaaa0000-0000-5000-8000-0000000000aa"; }
    }
}
`)
	file := source.New("declared.sysml", data)
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("the declared ElementId does not parse: %v", p.Diagnostics)
	}
	graph, err := ToRDFWith(file, root, IDUUID)
	if err != nil {
		t.Fatalf("ToRDFWith: %v", err)
	}
	document, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatalf("WriteAPIJSON: %v", err)
	}
	back := decodeAPIJSON(t, document)
	if !strings.Contains(string(back), `id = "aaaa0000-0000-5000-8000-0000000000aa"`) {
		t.Fatalf("the declared ElementId was dropped:\n%s", back)
	}
}
