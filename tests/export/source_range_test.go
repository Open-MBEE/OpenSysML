package export_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// Every element the encoder writes from a syntax node with a qualified name
// carries sysx:sourceRange, the 1-based byte positions of the declaration's own
// notation, and each root carries sysx:sourceDocument, the file it was read
// from. Both are provenance: they move wherever a hop's notation placed the
// element, and the reader ignores them.
func TestSourceRangeCoversEachDeclaration(t *testing.T) {
	src := "package W {\n\tpart a { attribute x; }\n\tpart b { attribute x; }\n}\n"
	turtle, err := convert.Convert("w.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	for _, want := range []string{
		`elmt:W
    a sysml:Package ;
    sysml:qualifiedName "W" ;
    sysx:sourceRange "1:1-4:2" ;`,
		`sysx:sourceDocument "w.sysml" ;`,
		`sysml:qualifiedName "W::a" ;
    sysx:sourceRange "2:2-2:25" ;`,
		`sysml:qualifiedName "W::a::x" ;
    sysx:sourceRange "2:11-2:23" ;`,
		`sysml:qualifiedName "W::b::x" ;
    sysx:sourceRange "3:11-3:23" ;`,
	} {
		if !strings.Contains(string(turtle), want) {
			t.Errorf("the graph should carry %q\n%s", want, turtle)
		}
	}

	// The API's element form states both as plain sysx: string properties.
	document, err := convert.Convert("w.sysml", []byte(src), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("to api-json: %v", err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatalf("the api-json does not parse: %v", err)
	}
	var root bool
	var ranges map[string]string = map[string]string{}
	for _, el := range elements {
		if el["sysx:sourceRange"] != nil {
			ranges[el["@id"].(string)] = el["sysx:sourceRange"].(string)
		}
		if el["@id"] == "W" {
			root = el["sysx:sourceDocument"] == "w.sysml"
		}
	}
	for id, want := range map[string]string{
		"W":       "1:1-4:2",
		"W__a":    "2:2-2:25",
		"W__a__x": "2:11-2:23",
		"W__b__x": "3:11-3:23",
	} {
		if ranges[id] != want {
			t.Errorf("api-json %s sysx:sourceRange = %q, want %q", id, ranges[id], want)
		}
	}
	if !root {
		t.Errorf("the root should carry sysx:sourceDocument %q:\n%s", "w.sysml", document)
	}
}

// A range ends where the declaration's own notation does: a trailing comment
// and the trivia before it stay outside.
func TestSourceRangeStopsAtTheNotationsEnd(t *testing.T) {
	src := "package M {\n\tpart def Engine {\n\t\tattribute power : Real; // comment\n\t}\n}\n"
	turtle, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	for _, want := range []string{
		`sysml:qualifiedName "M::Engine" ;
    sysx:sourceRange "2:2-4:3" ;`,
		`sysml:qualifiedName "M::Engine::power" ;
    sysx:sourceRange "3:3-3:26" ;`,
	} {
		if !strings.Contains(string(turtle), want) {
			t.Errorf("the graph should carry %q\n%s", want, turtle)
		}
	}
}

// A KerML document's elements carry their range the same way.
func TestSourceRangeInKerML(t *testing.T) {
	src := "package K {\n\tclass C { feature f; }\n}\n"
	turtle, err := convert.Convert("k.kerml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	for _, want := range []string{
		`sysml:qualifiedName "K::C" ;
    sysx:sourceRange "2:2-2:24" ;`,
		`sysml:qualifiedName "K::C::f" ;
    sysx:sourceRange "2:12-2:22" ;`,
	} {
		if !strings.Contains(string(turtle), want) {
			t.Errorf("the graph should carry %q\n%s", want, turtle)
		}
	}
}

// Columns count bytes, as LineIndex.PosAt gives them: the é in 'héllo' shifts
// the end column by its two bytes, not one column.
func TestSourceRangeColumnsCountBytes(t *testing.T) {
	src := "package P { part 'héllo'; }\n"
	turtle, err := convert.Convert("u.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	want := `sysx:sourceRange "1:13-1:27" ;`
	if !strings.Contains(string(turtle), want) {
		t.Errorf("the é should count as two bytes of column:\nwant %q in\n%s", want, turtle)
	}
}

// The ranges and the document name are provenance the reader ignores: notation
// rebuilt from the graph alone reads the same whether they are stated or not,
// and the structural triple set is otherwise unchanged.
func TestSourceRangeAndDocumentAreProvenanceOnly(t *testing.T) {
	src := "package W {\n\tpart a { attribute x; }\n\tpart b { attribute x; }\n}\n"
	turtle, err := convert.Convert("w.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	withoutText := withoutTriples(t, withoutTriples(t, turtle, "sysx:sourceText"), "sysx:sourceTail")
	stripped := withoutProvenance(t, withoutText)
	back, err := convert.Convert("m.ttl", stripped, convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	withProvenance, err := convert.Convert("m.ttl", withoutText, convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation with the provenance: %v", err)
	}
	if string(back) != string(withProvenance) {
		t.Errorf("the provenance changed the notation:\n--- with ---\n%s--- without ---\n%s", withProvenance, back)
	}
	// Without the provenance the same notation parses, so the structural
	// triple set is unchanged.
	again, err := convert.Convert("w.sysml", back, convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle again: %v", err)
	}
	first, second := structuralTriples(t, withoutProvenance(t, turtle)), structuralTriples(t, withoutProvenance(t, again))
	for triple := range first {
		if !second[triple] {
			t.Errorf("the second hop lost %s %s %s", triple.Subject.Value, triple.Predicate.Value, triple.Object.Value)
		}
	}
	for triple := range second {
		if !first[triple] {
			t.Errorf("the second hop added %s %s %s", triple.Subject.Value, triple.Predicate.Value, triple.Object.Value)
		}
	}
}

// An annotation's `/* ... */` body is its declaration, not a trailing note: a
// doc's, a comment's and a rep's range runs to the end of the body, over
// however many lines it spans. A part's trailing comment still stays outside.
func TestSourceRangeCoversAnAnnotationsBody(t *testing.T) {
	src := "package A {\n\tpart x; // a note\n\tdoc /* one wheel */\n\tcomment c about x /* a note */\n\trep r language \"text\" /* a rep */\n\tdoc d /* a body\n\tover two lines */\n}\n"
	turtle, err := convert.Convert("a.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	for _, want := range []string{
		`sysml:qualifiedName "A::x" ;
    sysx:sourceRange "2:2-2:9" ;`,
		`sysx:sourceRange "3:2-3:21" ;`,
		`sysml:qualifiedName "A::c" ;
    sysx:sourceRange "4:2-4:32" ;`,
		`sysml:qualifiedName "A::r" ;
    sysx:sourceRange "5:2-5:35" ;`,
		`sysml:qualifiedName "A::d" ;
    sysx:sourceRange "6:2-7:19" ;`,
	} {
		if !strings.Contains(string(turtle), want) {
			t.Errorf("the graph should carry %q\n%s", want, turtle)
		}
	}

	document, err := convert.Convert("a.sysml", []byte(src), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("to api-json: %v", err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatalf("the api-json does not parse: %v", err)
	}
	var docRange string
	for _, el := range elements {
		if el["@id"] == "A__d" {
			docRange, _ = el["sysx:sourceRange"].(string)
		}
	}
	if docRange != "6:2-7:19" {
		t.Errorf("api-json doc range = %q, want %q", docRange, "6:2-7:19")
	}
}
