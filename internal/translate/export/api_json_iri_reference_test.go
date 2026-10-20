package export

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// TestReferenceSpelledAsExpressionIRIDecodes checks a reference a service hands
// back as the expression IRI itself — {"@id": "urn:opensysml:expr:<id>"}, as
// Flexo MMS delivers a FeatureValue's value — names the expression the bare id
// does, so the delivered model still re-converts to notation.
func TestReferenceSpelledAsExpressionIRIDecodes(t *testing.T) {
	const model = "package P {\n    attribute count : ScalarValues::Integer = 4;\n}\n"
	file := source.New("p.sysml", []byte(model))
	root := parser.New(file).ParseFile()
	graph, err := ToRDF(file, root)
	if err != nil {
		t.Fatalf("ToRDF: %v", err)
	}
	elements, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatalf("WriteAPIJSON: %v", err)
	}
	var objects []map[string]any
	if err := json.Unmarshal(elements, &objects); err != nil {
		t.Fatalf("decode the element form: %v", err)
	}
	expressions := map[string]bool{}
	for _, object := range objects {
		if object["@type"] == "LiteralInteger" {
			expressions[object["@id"].(string)] = true
			// The service keeps sysml: properties only; the structure has to carry the value.
			delete(object, "sysx:sourceText")
		}
	}
	if len(expressions) == 0 {
		t.Fatal("the model wrote no LiteralInteger")
	}
	respelled := 0
	respell := func(value any) {
		if ref, ok := value.(map[string]any); ok {
			if id, ok := ref["@id"].(string); ok && expressions[id] {
				ref["@id"] = rdf.Expression + id
				respelled++
			}
		}
	}
	for _, object := range objects {
		for _, value := range object {
			respell(value)
			if list, ok := value.([]any); ok {
				for _, item := range list {
					respell(item)
				}
			}
		}
	}
	if respelled == 0 {
		t.Fatal("no reference to the expression was respelled")
	}
	spelled, err := json.Marshal(objects)
	if err != nil {
		t.Fatal(err)
	}
	notation := decodeAPIJSON(t, spelled)
	if !strings.Contains(string(notation), "= 4") {
		t.Fatalf("the value did not survive the IRI-spelled reference:\n%s", notation)
	}
}

// TestShuffledElementSetDecodes checks the element set decodes however a
// service orders it: elements shuffled, every collection shuffled, the sysx:
// annotations gone, as a store keeping no order on ownedRelationship delivers
// it. The positional names (`P::@0`) state the members' places.
func TestShuffledElementSetDecodes(t *testing.T) {
	const model = "package P {\n    private import ISQ::*;\n    private import ScalarValues::*;\n" +
		"    part def V {\n        attribute m : ISQ::mass;\n        attribute n : ScalarValues::Integer;\n    }\n" +
		"    part v : V {\n        attribute redefines n = 4;\n    }\n}\n"
	file := source.New("p.sysml", []byte(model))
	root := parser.New(file).ParseFile()
	graph, err := ToRDFWith(file, root, IDUUID)
	if err != nil {
		t.Fatalf("ToRDF: %v", err)
	}
	elements, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatalf("WriteAPIJSON: %v", err)
	}
	var objects []map[string]any
	if err := json.Unmarshal(elements, &objects); err != nil {
		t.Fatalf("decode the element form: %v", err)
	}
	for _, object := range objects {
		for key, value := range object {
			if strings.HasPrefix(key, "sysx:") {
				delete(object, key)
			}
			if list, ok := value.([]any); ok {
				slices.Reverse(list)
			}
		}
	}
	for shuffle := 0; shuffle < 4; shuffle++ {
		slices.SortFunc(objects, func(a, b map[string]any) int {
			return strings.Compare(hashed(a["@id"].(string), shuffle), hashed(b["@id"].(string), shuffle))
		})
		shuffled, err := json.Marshal(objects)
		if err != nil {
			t.Fatal(err)
		}
		notation := string(decodeAPIJSON(t, shuffled))
		for _, want := range []string{"import ISQ::*", "import ScalarValues::*", "= 4"} {
			if !strings.Contains(notation, want) {
				t.Fatalf("order %d: the notation lacks %q:\n%s", shuffle, want, notation)
			}
		}
		// The imports let the writer shorten the library names; either spelling names the element.
		for _, want := range []*regexp.Regexp{
			regexp.MustCompile(`attribute m : (ISQ::|ISQBase::)?mass;`),
			regexp.MustCompile(`attribute n : (ScalarValues::)?Integer;`),
		} {
			if !want.MatchString(notation) {
				t.Fatalf("order %d: the notation lacks %s:\n%s", shuffle, want, notation)
			}
		}
	}
}

// hashed is a deterministic reordering key for one shuffle.
func hashed(id string, shuffle int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", shuffle, id)))
	return fmt.Sprintf("%x", sum)
}
