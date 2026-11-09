package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// A model whose notes — `//` and `//* */` notes, and `/* */` comments where the
// grammar admits no Comment element — lie in heads, in an expression and in a body's tail.
const noted = `// The rover, as modelled.
package Rover {
    part def Wheel :> Part; // a synonym the printer would spell out
    //* A note the parser skips
       over two lines. */
    part def Hub;
    part def Vehicle {
        doc /* what a vehicle is for */
        part wheels : Wheel[4]; // four of them
        attribute x = 1 /* one */ + 2; // sum
        // connected at the hub
        connect wheels to hub;
        // closing
    }
}
`

// notedCanonical is noted rebuilt from the graph alone, its notes put back after
// the tokens they followed; the synonym is spelled out, as the graph has it.
const notedCanonical = `// The rover, as modelled.
package Rover {
    part def Wheel specializes Part; // a synonym the printer would spell out
    //* A note the parser skips
       over two lines. */
    part def Hub;
    part def Vehicle {
        doc /* what a vehicle is for */
        part wheels : Wheel[4]; // four of them
        attribute x = 1 /* one */ + 2; // sum
        // connected at the hub
        connect wheels to hub;
        // closing
    }
}
`

// withoutSourceTextKeepingNotes strips the source text and nothing else, so the
// notes alone carry the comments back.
func withoutSourceTextKeepingNotes(t *testing.T, turtle []byte) []byte {
	t.Helper()
	for _, property := range append([]string{"sysx:sourceText", "sysx:sourceTail", "sysx:sourceLanguage"}, sourceRangeProperties...) {
		turtle = withoutProperty(t, turtle, property)
	}
	return []byte(withoutLibraryStubs(string(turtle)))
}

func TestNotesAreNodesOfTheirElement(t *testing.T) {
	turtle := string(idTurtle(t, noted))
	for _, want := range []string{
		// A leading note on its own line, ahead of the root's first token.
		"expr:Rover_pnote0\n    a sysx:Note ;\n    sysx:noteText \"// The rover, as modelled.\" ;\n    sysx:noteToken \"0\"^^xsd:integer ;\n    sysx:noteOwnLine \"true\"^^xsd:boolean .",
		// An inline note after the head's last token, which no count places.
		"expr:Rover__Wheel_pnote0\n    a sysx:Note ;\n    sysx:noteText \"// a synonym the printer would spell out\" ;\n    sysx:noteTrailing \"true\"^^xsd:boolean ;\n    sysx:noteOwnLine \"false\"^^xsd:boolean .",
		// A multi-line note, as written.
		"sysx:noteText \"//* A note the parser skips\\n       over two lines. */\" ;\n    sysx:noteToken \"0\"^^xsd:integer ;\n    sysx:noteOwnLine \"true\"^^xsd:boolean .",
		// Two notes of one declaration, one inside its expression, in order.
		"    sysx:note expr:Rover__Vehicle__x_pnote0, expr:Rover__Vehicle__x_pnote1 ;",
		"expr:Rover__Vehicle__x_pnote0\n    a sysx:Note ;\n    sysx:noteText \"/* one */\" ;\n    sysx:noteToken \"4\"^^xsd:integer ;\n    sysx:noteOwnLine \"false\"^^xsd:boolean .",
		"expr:Rover__Vehicle__x_pnote1\n    a sysx:Note ;\n    sysx:noteText \"// sum\" ;\n    sysx:noteTrailing \"true\"^^xsd:boolean ;\n    sysx:noteOwnLine \"false\"^^xsd:boolean .",
		// A note in the tail closing a body, ahead of its `}`.
		"expr:Rover__Vehicle_pnote0\n    a sysx:Note ;\n    sysx:noteText \"// closing\" ;\n    sysx:noteToken \"0\"^^xsd:integer ;\n    sysx:noteOwnLine \"true\"^^xsd:boolean ;\n    sysx:noteTail \"true\"^^xsd:boolean .",
	} {
		if !strings.Contains(turtle, want) {
			t.Errorf("the graph lacks:\n%s\n--- graph ---\n%s", want, turtle)
		}
	}
	// The doc body is the Documentation's, not a note; a note is no element.
	for _, unwanted := range []string{`sysx:noteText "/* what a vehicle is for */"`, "expr:Rover_pnote0\n    a sysx:Note ;\n    sysml:elementId"} {
		if strings.Contains(turtle, unwanted) {
			t.Errorf("the graph states %q", unwanted)
		}
	}
	if got := strings.Count(turtle, "a sysx:Note ;"); got != 8 {
		t.Errorf("%d notes, want 8:\n%s", got, turtle)
	}
}

// Without source text the notation is rebuilt from the graph, notes included,
// and states the same graph again, notes included.
func TestNotesComeBackWithoutSourceText(t *testing.T) {
	turtle := idTurtle(t, noted)
	stripped := withoutSourceTextKeepingNotes(t, turtle)
	back := toNotation(t, stripped)
	if back != notedCanonical {
		t.Errorf("the notes were not put back:\n--- want ---\n%s--- got ---\n%s", notedCanonical, back)
	}
	if got, want := string(withoutSourceTextKeepingNotes(t, idTurtle(t, back))), string(stripped); got != want {
		t.Errorf("the rebuilt notation states another graph:\n--- want ---\n%s--- got ---\n%s", want, got)
	}
}

// An element rebuilt after a graph edit keeps the notes of its own lines where
// they were; its neighbours stay as written.
func TestEditedElementKeepsItsNotes(t *testing.T) {
	turtle := idTurtle(t, noted)
	edited := editTurtle(t, turtle, "elmt:Rover__Wheel\n    a sysml:PartDefinition ;\n", "elmt:Rover__Wheel\n    a sysml:ItemDefinition ;\n")
	want := strings.Replace(noted, "    part def Wheel :> Part; // a synonym the printer would spell out\n",
		"    item def Wheel specializes Part; // a synonym the printer would spell out\n", 1)
	if back := toNotation(t, edited); back != want {
		t.Errorf("the edited element lost its note:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
	// A body's tail note stays ahead of the `}` when the owner is rebuilt.
	edited = editTurtle(t, turtle, "elmt:Rover__Vehicle\n    a sysml:PartDefinition ;\n", "elmt:Rover__Vehicle\n    a sysml:ItemDefinition ;\n")
	want = strings.Replace(noted, "    part def Vehicle {\n", "    item def Vehicle {\n", 1)
	if back := toNotation(t, edited); back != want {
		t.Errorf("the rebuilt body lost its tail note:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
}

// The graph is authoritative over the text: a note edited or added there demotes
// the element's text, and the element is rebuilt with the graph's notes.
func TestNoteEditedInTheGraphIsWritten(t *testing.T) {
	turtle := idTurtle(t, noted)
	edited := editTurtle(t, turtle, `sysx:noteText "// four of them"`, `sysx:noteText "// all four"`)
	want := strings.Replace(noted, "// four of them", "// all four", 1)
	if back := toNotation(t, edited); back != want {
		t.Errorf("the edited note was not written:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
	// A note removed from the graph alone is kept by the text, which still
	// states it: a comment the graph does not state may be another subject's.
	removed := editTurtle(t, turtle, "    sysx:note expr:Rover__Vehicle__wheels_pnote0 ;\n", "")
	if back := toNotation(t, removed); back != noted {
		t.Errorf("the text was not kept:\n--- want ---\n%s--- got ---\n%s", noted, back)
	}
	// Without the text, the removed note is gone.
	want = strings.Replace(notedCanonical, " // four of them", "", 1)
	if back := toNotation(t, withoutSourceTextKeepingNotes(t, removed)); back != want {
		t.Errorf("the removed note was still written:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
}

// API JSON carries each note as a sysx:Note object of its own, addressable by
// id, and the element's sysx:note references in order.
func TestNotesSurviveAPIJSON(t *testing.T) {
	document, err := convert.Convert("m.sysml", []byte(noted), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("to api-json: %v", err)
	}
	text := string(document)
	for _, want := range []string{
		`"@type": "sysx:Note"`, `"@id": "Rover__Vehicle__x_pnote1"`, `"sysx:noteText": "// sum"`, `"sysx:noteTrailing": true`, `"sysx:noteToken": 4`,
		`"sysx:noteTail": true`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the api-json lacks %s:\n%s", want, text)
		}
	}
	turtle, err := convert.Convert("m.json", document, convert.FormatAPIJSON, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("api-json to turtle: %v", err)
	}
	if back := toNotation(t, turtle); back != noted {
		t.Errorf("the api-json round trip changed the notation:\n--- want ---\n%s--- got ---\n%s", noted, back)
	}
	if back := toNotation(t, withoutSourceTextKeepingNotes(t, turtle)); back != notedCanonical {
		t.Errorf("the notes did not come back through api-json without source text:\n--- want ---\n%s--- got ---\n%s", notedCanonical, back)
	}
}

// withoutNotes drops the sysx:Note nodes and the sysx:note statements naming them.
func withoutNotes(t *testing.T, turtle []byte) []byte {
	t.Helper()
	var blocks []string
	for _, block := range strings.Split(string(turtle), "\n\n") {
		if strings.Contains(block, "\n    a sysx:Note ;\n") {
			continue
		}
		blocks = append(blocks, block)
	}
	out := string(withoutProperty(t, []byte(strings.Join(blocks, "\n\n")), "sysx:note"))
	if strings.HasSuffix(string(turtle), "\n") {
		out = strings.TrimRight(out, "\n") + "\n"
	}
	// The notes may have been the only expression-namespace nodes.
	const prefix = "@prefix expr: <urn:opensysml:expr:> .\n"
	if !strings.Contains(strings.Replace(out, prefix, "", 1), "expr:") {
		out = strings.Replace(out, prefix, "", 1)
	}
	return []byte(out)
}

// withoutProperty drops the statements of one property, and nothing else.
func withoutProperty(t *testing.T, turtle []byte, property string) []byte {
	t.Helper()
	var blocks []string
	for _, block := range strings.Split(string(turtle), "\n\n") {
		var kept []string
		for _, line := range strings.Split(block, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), property+" ") {
				continue
			}
			kept = append(kept, line)
		}
		// Dropping the last triple of a block leaves the one before it to end it.
		for i := len(kept) - 1; i >= 0; i-- {
			if trimmed := strings.TrimRight(kept[i], "\n"); strings.HasSuffix(trimmed, " ;") {
				kept[i] = strings.TrimSuffix(trimmed, " ;") + " ."
				break
			} else if trimmed != "" {
				break
			}
		}
		blocks = append(blocks, strings.Join(kept, "\n"))
	}
	return []byte(strings.Join(blocks, "\n\n"))
}

// A note inside an expression is held by the expression's text, which a rebuilt
// declaration keeps: it is not written a second time from the sysx:Note.
func TestNoteInsideAnExpressionIsWrittenOnce(t *testing.T) {
	turtle := idTurtle(t, noted)
	edited := editTurtle(t, turtle, `sysml:declaredName "x"`, `sysml:declaredName "y"`)
	edited = editTurtle(t, edited, `sysml:qualifiedName "Rover::Vehicle::x"`, `sysml:qualifiedName "Rover::Vehicle::y"`)
	back := toNotation(t, edited)
	for _, note := range []string{"/* one */", "// sum"} {
		if n := strings.Count(back, note); n != 1 {
			t.Errorf("the rebuilt declaration writes %s %d time(s):\n%s", note, n, back)
		}
	}
	if !strings.Contains(back, "attribute y = 1 /* one */ + 2 {") {
		t.Errorf("the rebuilt declaration lost its expression's note:\n%s", back)
	}
}

// A note the graph names without a text cannot be written back, and is refused
// rather than dropped.
func TestNoteWithoutTextIsRefused(t *testing.T) {
	broken := withoutProperty(t, idTurtle(t, noted), "sysx:noteText")
	_, err := convert.Convert("m.ttl", broken, convert.FormatTurtle, convert.FormatSysML)
	if err == nil || !strings.Contains(err.Error(), "sysx:noteText") {
		t.Fatalf("a note without text was not refused: %v", err)
	}
}
