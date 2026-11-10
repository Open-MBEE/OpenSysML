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

// A model whose notes lie at positions a graph edit can move: inside a head
// outside any expression, trailing a declaration, and in a body's tail.
const positioned = `package Fleet {
    part def Axle /* the shaft */ :> Part;
    part def Truck {
        part axles : Axle[2]; // two of them
        attribute load : Real;
        // closing
    }
    part def Trailer;
}
`

// positionEdited converts positioned to Turtle, applies one edit to a note's
// position, converts back and compares to the notation expected.
func positionEdited(t *testing.T, old, replacement, want string) {
	t.Helper()
	turtle := idTurtle(t, positioned)
	back := toNotation(t, editTurtle(t, turtle, old, replacement))
	if back != want {
		t.Errorf("the position edit was not honored:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
	// The rebuilt notation states the graph's notes, so a second conversion keeps it.
	if again := toNotation(t, idTurtle(t, back)); again != back {
		t.Errorf("the rebuilt notation does not stand:\n--- want ---\n%s--- got ---\n%s", back, again)
	}
}

// A note's position edited alone (token, trailing, line, head or tail) demotes the
// element's text, which is rebuilt with the note where the graph puts it; neighbours stand.
func TestNotePositionEditedInTheGraphIsWritten(t *testing.T) {
	const axle = "expr:Fleet__Axle_pnote0\n    a sysx:Note ;\n    sysx:noteText \"/* the shaft */\" ;\n    sysx:noteToken \"3\"^^xsd:integer ;"
	const axles = "expr:Fleet__Truck__axles_pnote0\n    a sysx:Note ;\n    sysx:noteText \"// two of them\" ;\n    sysx:noteTrailing \"true\"^^xsd:boolean ;\n    sysx:noteOwnLine \"false\"^^xsd:boolean ."
	const closing = "expr:Fleet__Truck_pnote0\n    a sysx:Note ;\n    sysx:noteText \"// closing\" ;\n    sysx:noteToken \"0\"^^xsd:integer ;\n    sysx:noteOwnLine \"true\"^^xsd:boolean ;\n    sysx:noteTail \"true\"^^xsd:boolean ."
	t.Run("token", func(t *testing.T) {
		// An inline note moved to another of the element's own tokens.
		positionEdited(t, axle, strings.Replace(axle, `"3"^^xsd:integer`, `"2"^^xsd:integer`, 1),
			strings.Replace(positioned, "    part def Axle /* the shaft */ :> Part;\n", "    part def /* the shaft */ Axle specializes Part;\n", 1))
	})
	t.Run("trailing", func(t *testing.T) {
		// A trailing note given a token count follows that token instead.
		positionEdited(t, axles, strings.Replace(axles, `sysx:noteTrailing "true"^^xsd:boolean`, `sysx:noteToken "2"^^xsd:integer`, 1),
			strings.Replace(positioned, "        part axles : Axle[2]; // two of them\n", "        part axles // two of them\n            : Axle[2];\n", 1))
	})
	t.Run("own line", func(t *testing.T) {
		// An inline note put on its own line, after the declaration it trails.
		positionEdited(t, axles, strings.Replace(axles, `sysx:noteOwnLine "false"`, `sysx:noteOwnLine "true"`, 1),
			strings.Replace(positioned, "        part axles : Axle[2]; // two of them\n", "        part axles : Axle[2];\n        // two of them\n", 1))
	})
	t.Run("inline", func(t *testing.T) {
		// A note on its own line put inline, ahead of the token it preceded.
		positionEdited(t, axle, strings.Replace(axle, `sysx:noteToken "3"^^xsd:integer ;`, `sysx:noteToken "0"^^xsd:integer ;`, 1),
			strings.Replace(positioned, "    part def Axle /* the shaft */ :> Part;\n", "    /* the shaft */ part def Axle specializes Part;\n", 1))
	})
	t.Run("tail", func(t *testing.T) {
		// A tail note moved to the head goes above the declaration; the body's
		// members and its `}` stand.
		positionEdited(t, closing, strings.Replace(closing, " ;\n    sysx:noteTail \"true\"^^xsd:boolean .", " .", 1),
			strings.Replace(strings.Replace(positioned, "        // closing\n    }\n", "    }\n", 1), "    part def Truck {\n", "    // closing\n    part def Truck {\n", 1))
	})
}

// Two notes at one token are told apart by their order alone, so their texts swapped
// in the graph demote the text as any other edit does, and both are written swapped.
func TestNotesAtOneTokenSwappedInTheGraphAreWritten(t *testing.T) {
	const twice = `package Fleet {
    part def Truck {
        // first
        // second
        part axles : Axle[2];
    }
}
`
	turtle := idTurtle(t, twice)
	edited := editTurtle(t, turtle, `sysx:noteText "// first"`, `sysx:noteText "// swapped"`)
	edited = editTurtle(t, edited, `sysx:noteText "// second"`, `sysx:noteText "// first"`)
	edited = editTurtle(t, edited, `sysx:noteText "// swapped"`, `sysx:noteText "// second"`)
	want := strings.Replace(twice, "        // first\n        // second\n", "        // second\n        // first\n", 1)
	if back := toNotation(t, edited); back != want {
		t.Errorf("the swapped notes were not written:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
}

// A note inside an expression moved in the graph is written where the graph puts
// it: the expression's text, which would hold it where it was, is rebuilt too.
func TestNoteInsideAnExpressionMovedInTheGraphIsWritten(t *testing.T) {
	const one = "    sysx:noteText \"/* one */\" ;\n    sysx:noteToken \"4\"^^xsd:integer ;"
	edited := editTurtle(t, idTurtle(t, noted), one, strings.Replace(one, `"4"`, `"3"`, 1))
	want := strings.Replace(noted, "        attribute x = 1 /* one */ + 2; // sum\n", "        attribute x = /* one */ 1 + 2; // sum\n", 1)
	back := toNotation(t, edited)
	if back != want {
		t.Errorf("the moved note was not written:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
	if again := toNotation(t, idTurtle(t, back)); again != back {
		t.Errorf("the rebuilt notation does not stand:\n--- want ---\n%s--- got ---\n%s", back, again)
	}
}

// A `//` note the graph puts inline ahead of a declaration's first token would run
// on over the declaration, so it takes its own line, where it reads as the note it is.
func TestLineNoteAheadOfTheDeclarationKeepsItsLine(t *testing.T) {
	turtle := idTurtle(t, noted)
	const hub = "    sysx:noteText \"// connected at the hub\" ;\n    sysx:noteToken \"0\"^^xsd:integer ;\n    sysx:noteOwnLine \"true\"^^xsd:boolean ."
	edited := editTurtle(t, turtle, hub, strings.Replace(hub, `"true"`, `"false"`, 1))
	if back := toNotation(t, edited); back != noted {
		t.Errorf("the note did not keep its line:\n--- want ---\n%s--- got ---\n%s", noted, back)
	}
}

// A graph whose notes state what the text holds, position included, writes the
// text byte for byte; one edited elsewhere rebuilds that element alone.
func TestAgreeingNotesKeepTheText(t *testing.T) {
	turtle := idTurtle(t, positioned)
	if back := toNotation(t, turtle); back != positioned {
		t.Errorf("the unchanged graph did not write the text as it was:\n--- want ---\n%s--- got ---\n%s", positioned, back)
	}
	edited := editTurtle(t, turtle, "elmt:Fleet__Trailer\n    a sysml:PartDefinition ;\n", "elmt:Fleet__Trailer\n    a sysml:ItemDefinition ;\n")
	want := strings.Replace(positioned, "    part def Trailer;\n", "    item def Trailer;\n", 1)
	if back := toNotation(t, edited); back != want {
		t.Errorf("the neighbours of the rebuilt element did not keep their text:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
}
