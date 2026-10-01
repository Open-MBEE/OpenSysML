package convert_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// The API element form parses by its names, infers from a .json path, prints
// as api-json, and is listed among the formats.
func TestAPIJSONIsAFormat(t *testing.T) {
	for _, name := range []string{"api-json", "json"} {
		f, err := convert.ParseFormat(name)
		if err != nil || f != convert.FormatAPIJSON {
			t.Errorf("ParseFormat(%q) = %v, %v, want FormatAPIJSON", name, f, err)
		}
	}
	if got := convert.FormatAPIJSON.String(); got != "api-json" {
		t.Errorf("FormatAPIJSON.String() = %q", got)
	}
	f, err := convert.FormatOfPath("model.json")
	if err != nil || f != convert.FormatAPIJSON {
		t.Errorf("FormatOfPath(model.json) = %v, %v", f, err)
	}
	if !strings.Contains(convert.FormatList, "api-json") {
		t.Errorf("FormatList does not name api-json: %s", convert.FormatList)
	}
}

// A model converts to the element form and back through Convert the same as by
// hand, and the two JSON directions compose as Turtle's do.
func TestAPIJSONConvertRoutes(t *testing.T) {
	text := []byte("package P { part def Q { attribute mass : Real = 3.0; } }")
	document, err := convert.Convert("p.sysml", text, convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("sysml to api-json: %v", err)
	}
	graph, err := convert.SysMLToRDF("p.sysml", text)
	if err != nil {
		t.Fatalf("SysMLToRDF: %v", err)
	}
	turtle := rdf.WriteTurtle(graph)

	// api-json to sysml writes the same notation as ttl to sysml.
	fromJSON, err := convert.Convert("p.json", document, convert.FormatAPIJSON, convert.FormatSysML)
	if err != nil {
		t.Fatalf("api-json to sysml: %v", err)
	}
	fromTTL, err := convert.Convert("p.ttl", turtle, convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("ttl to sysml: %v", err)
	}
	if string(fromJSON) != string(fromTTL) {
		t.Errorf("api-json and ttl read back different notation:\n%s\n---\n%s", fromJSON, fromTTL)
	}

	// api-json to ttl spells the same graph as the Turtle the model wrote.
	asTTL, err := convert.Convert("p.json", document, convert.FormatAPIJSON, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("api-json to ttl: %v", err)
	}
	if string(asTTL) != string(turtle) {
		t.Errorf("api-json to ttl differs:\n%s\n---\n%s", asTTL, turtle)
	}

	// ttl to api-json spells the same document as sysml to api-json.
	fromTTLObj, err := convert.Convert("p.ttl", turtle, convert.FormatTurtle, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("ttl to api-json: %v", err)
	}
	if string(fromTTLObj) != string(document) {
		t.Errorf("ttl to api-json differs:\n%s\n---\n%s", fromTTLObj, document)
	}

	// api-json to api-json normalizes like ttl to ttl.
	again, err := convert.Convert("p.json", document, convert.FormatAPIJSON, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("api-json to api-json: %v", err)
	}
	if string(again) != string(document) {
		t.Errorf("api-json was not rewritten the same way:\n%s\n---\n%s", document, again)
	}
}

// The compact element document sysml-toolkit writes — first-class membership
// elements and one-member arrays on multi-valued properties — reads back to
// notation.
func TestAPIJSONToolkitCompactFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/toolkit_compact_package.json")
	if err != nil {
		t.Fatal(err)
	}
	text, err := convert.Convert("toolkit_compact_package.json", data, convert.FormatAPIJSON, convert.FormatSysML)
	if err != nil {
		t.Fatalf("api-json to sysml: %v", err)
	}
	for _, want := range []string{"package P", "part def V"} {
		if !strings.Contains(string(text), want) {
			t.Errorf("the notation does not contain %q:\n%s", want, text)
		}
	}
}

// A malformed element document is a syntax error of the input, as a malformed
// Turtle document is.
func TestAPIJSONSyntaxError(t *testing.T) {
	var syntax *convert.SyntaxError
	for _, to := range []convert.Format{convert.FormatSysML, convert.FormatTurtle, convert.FormatAPIJSON} {
		if _, err := convert.Convert("p.json", []byte(`{`), convert.FormatAPIJSON, to); !errors.As(err, &syntax) {
			t.Errorf("Convert to %s = %v, want a SyntaxError", to, err)
		}
	}
	repeated := `[{"@type": "Package", "@id": "X", "ownedMember": [{"@id": "A"}, {"@id": "A"}]}]`
	if _, err := convert.Convert("p.json", []byte(repeated), convert.FormatAPIJSON, convert.FormatTurtle); err == nil {
		t.Error("a collection repeating a member converted to Turtle, want an error")
	}
}

// The element form goes through the RDF mapping, so it carries the mapping's
// experimental status in either direction.
func TestAPIJSONIsExperimental(t *testing.T) {
	for _, pair := range [][2]convert.Format{
		{convert.FormatSysML, convert.FormatAPIJSON},
		{convert.FormatAPIJSON, convert.FormatSysML},
		{convert.FormatAPIJSON, convert.FormatAPIJSON},
	} {
		if !convert.IsExperimental(pair[0], pair[1]) {
			t.Errorf("IsExperimental(%s, %s) = false", pair[0], pair[1])
		}
		notices := convert.Notices(pair[0], pair[1])
		if len(notices) != 1 || notices[0] != convert.ExperimentalNotice {
			t.Errorf("Notices(%s, %s) = %v, want the RDF notice", pair[0], pair[1], notices)
		}
	}
}
