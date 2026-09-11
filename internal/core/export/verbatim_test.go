package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
)

// A model whose graph carries the notation itself as source text: comments,
// notes, blank lines and synonyms included.
const commented = `// The rover, as modelled.
package Rover {
    /* Definitions come first. */
    part def Wheel :> Part; // a synonym the printer would spell out
    //* A note the parser skips
       over two lines. */
    part def Hub;

    part def Vehicle {
        doc /* what a vehicle is for */
        part wheels : Wheel[4]; // four of them
        part hub : Hub;
        // connected at the hub
        connect wheels to hub;
    }
}
`

// commentedTurtle converts commented to Turtle.
func commentedTurtle(t *testing.T) []byte {
	t.Helper()
	return idTurtle(t, commented)
}

// editTurtle rewrites one line of a Turtle document, failing when it is absent.
func editTurtle(t *testing.T, turtle []byte, old, replacement string) []byte {
	t.Helper()
	if !strings.Contains(string(turtle), old) {
		t.Fatalf("%q is not in the graph:\n%s", old, turtle)
	}
	return []byte(strings.Replace(string(turtle), old, replacement, 1))
}

func TestSourceTextComesBackByteForByte(t *testing.T) {
	turtle := commentedTurtle(t)
	if back := toNotation(t, turtle); back != commented {
		t.Errorf("round trip changed the notation:\n--- want ---\n%s--- got ---\n%s", commented, back)
	}
	// Unformatted notation comes back as written too: the graph carries the
	// author's bytes, not the formatter's.
	loose := strings.ReplaceAll(commented, "    ", "\t")
	if back := toNotation(t, idTurtle(t, loose)); back != loose {
		t.Errorf("round trip changed the unformatted notation:\n--- want ---\n%s--- got ---\n%s", loose, back)
	}
}

// What follows the last root — notes, comments, blank lines, or no final
// newline — is the last root's tail, since the document itself has no subject.
func TestTrailingTriviaComesBack(t *testing.T) {
	for _, tail := range []string{"\n\n// the end\n\n/* really */\n\n\n", "\n\n", ""} {
		src := strings.TrimSuffix(commented, "\n") + tail
		if back := toNotation(t, idTurtle(t, src)); back != src {
			t.Errorf("tail %q: round trip changed the notation:\n--- want ---\n%s--- got ---\n%s", tail, src, back)
		}
	}
}

// Roots have no owner to be written whole with, so roots sharing a line each
// carry their slice of it, trivia between them included.
func TestRootsSharingALineComeBack(t *testing.T) {
	src := "package A; /* between */ package B; // after\n\npackage C {\n\tpart p; } package D;"
	turtle := idTurtle(t, src)
	if back := toNotation(t, turtle); back != src {
		t.Errorf("round trip changed the notation:\n--- want ---\n%s--- got ---\n%s", src, back)
	}
	// An edited root is rebuilt where it stood, its trailing note with it; its
	// neighbours stay as written.
	edited := editTurtle(t, turtle,
		"    sysml:declaredName \"B\" ;\n",
		"    sysml:declaredName \"B\" ;\n    sysx:isLibraryPackage \"true\"^^xsd:boolean ;\n")
	back := toNotation(t, edited)
	want := "package A; /* between */ library package B;\n\npackage C {\n\tpart p; } package D;"
	if back != want {
		t.Errorf("the edited root was not rebuilt in place:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
	if got, want := structural(t, idTurtle(t, back)), structural(t, edited); got != want {
		t.Errorf("the notation does not state the edited graph:\n--- want ---\n%s--- got ---\n%s", want, got)
	}
	// The trivia after a root is its own, so a root rebuilt ahead of another
	// leaves that one at the start of its line.
	edited = editTurtle(t, turtle,
		"    sysml:declaredName \"A\" ;\n",
		"    sysml:declaredName \"A\" ;\n    sysx:isLibraryPackage \"true\"^^xsd:boolean ;\n")
	want = "library package A;\npackage B; // after\n\npackage C {\n\tpart p; } package D;"
	if back := toNotation(t, edited); back != want {
		t.Errorf("the first root was not rebuilt in place:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
}

// Without source text the graph converts to canonical notation as before: the
// structural triples alone carry the model, and the trivia is gone.
func TestStrippedGraphPrintsCanonically(t *testing.T) {
	back := toNotation(t, withoutTriples(t, commentedTurtle(t), "sysx:sourceText"))
	want := `package Rover {
    part def Wheel specializes Part;
    part def Hub;
    part def Vehicle {
        doc /* what a vehicle is for */
        part wheels : Wheel[4];
        part hub : Hub;
        connect wheels to hub;
    }
}
`
	if back != want {
		t.Errorf("canonical notation changed:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
}

// An edit after export wins over the stale text of the member it touched, and
// only that member's lines are replaced with canonical notation; the blank line
// ahead of the next member is that member's own.
func TestEditedGraphDoesNotResurrectStaleText(t *testing.T) {
	turtle := editTurtle(t, commentedTurtle(t),
		"    sysml:declaredName \"Hub\" ;\n",
		"    sysml:declaredName \"Hub\" ;\n    sysml:isAbstract \"true\"^^xsd:boolean ;\n")
	back := toNotation(t, turtle)
	want := `// The rover, as modelled.
package Rover {
    /* Definitions come first. */
    part def Wheel :> Part; // a synonym the printer would spell out
    abstract part def Hub;

    part def Vehicle {
        doc /* what a vehicle is for */
        part wheels : Wheel[4]; // four of them
        part hub : Hub;
        // connected at the hub
        connect wheels to hub;
    }
}
`
	if back != want {
		t.Errorf("stale text was not replaced by the edit:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
	// Converting the result again reproduces the edited graph, so the notation
	// states exactly what the graph states.
	again := idTurtle(t, back)
	if got, want := withoutTriples(t, again, "sysx:sourceText"), withoutTriples(t, turtle, "sysx:sourceText"); string(got) != string(want) {
		t.Errorf("the notation does not state the edited graph:\n--- want ---\n%s--- got ---\n%s", want, got)
	}
}

// A removed member takes its text with it: the owner's own text does not
// state its members, so the owner and the remaining member print as written.
func TestRemovedMemberIsNotResurrected(t *testing.T) {
	turtle := idTurtle(t, `package P {
    part def A; // the first
    part def B; // the second
}
`)
	back := toNotation(t, withoutMember(t, turtle, "elmt:P__B"))
	want := "package P {\n    part def A; // the first\n}\n"
	if back != want {
		t.Errorf("the removed member came back:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
}

// Removing a member from the middle of a body leaves the members after it
// numbered as they were; their text, comments and spacing still stand.
func TestMembersAfterARemovedOneKeepTheirText(t *testing.T) {
	turtle := idTurtle(t, `package P {
    part def A; // the first
    part def B; // the second
    /* about C */
    part def C {
        part x : A; // inside
    }

    part def D; // the fourth
}
`)
	turtle = withoutMember(t, turtle, "elmt:P__B")
	back := toNotation(t, turtle)
	want := `package P {
    part def A; // the first
    /* about C */
    part def C {
        part x : A; // inside
    }

    part def D; // the fourth
}
`
	if back != want {
		t.Errorf("members after the removed one lost their text:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
	// The members after the removed one are renumbered on the way back, but
	// otherwise the notation states exactly what the graph states.
	renumbered := func(turtle []byte) string {
		return string(withoutTriples(t, withoutTriples(t, turtle, "sysx:sourceText"), "sysx:memberIndex"))
	}
	if got, want := renumbered(idTurtle(t, back)), renumbered(turtle); got != want {
		t.Errorf("the notation does not state the edited graph:\n--- want ---\n%s--- got ---\n%s", want, got)
	}
}

// The blank line ahead of a member is its own: it stays when the member before
// it is rebuilt or removed, whichever newline the file is written with.
func TestBlankLinesStayWithTheMemberAfterThem(t *testing.T) {
	for name, nl := range map[string]string{"LF": "\n", "CRLF": "\r\n"} {
		t.Run(name, func(t *testing.T) {
			lines := func(text string) string { return strings.ReplaceAll(text, "\n", nl) }
			notation := lines(`package P {
    part def A;

    // about B
    part def B {

        part x : A;
    }

    part def C; // the third
}
`)
			if back := toNotation(t, idTurtle(t, notation)); back != notation {
				t.Errorf("the notation changed:\n--- want ---\n%s--- got ---\n%s", notation, back)
			}
			edited := editTurtle(t, idTurtle(t, notation),
				"    sysml:declaredName \"A\" ;\n",
				"    sysml:declaredName \"A\" ;\n    sysml:isAbstract \"true\"^^xsd:boolean ;\n")
			want := strings.Replace(notation, "    part def A;", "    abstract part def A;", 1)
			if back := toNotation(t, edited); back != want {
				t.Errorf("rebuilding a member took the blank line after it:\n--- want ---\n%s--- got ---\n%s", want, back)
			}
			want = lines(`package P {
    part def A;

    part def C; // the third
}
`)
			if back := toNotation(t, withoutMember(t, idTurtle(t, notation), "elmt:P__B")); back != want {
				t.Errorf("removing a member took the blank line after it:\n--- want ---\n%s--- got ---\n%s", want, back)
			}
		})
	}
}

// The line ending rebuilt notation is written with is the one most of the file
// uses. An expression's text lies inside its member's, so however deeply it
// nests, the lines it spans count once.
func TestNestedExpressionTextDoesNotOutvoteTheFileNewline(t *testing.T) {
	notation := "package P {\n" +
		"    part def A;\n" +
		"    attribute x = (1 +\r\n" +
		"        (2 *\r\n" +
		"        (3 +\r\n" +
		"        4)));\n" +
		"    part def B;\n" +
		"}\n"
	turtle := idTurtle(t, notation)
	if back := toNotation(t, turtle); back != notation {
		t.Errorf("the notation changed:\n--- want ---\n%q--- got ---\n%q", notation, back)
	}
	edited := editTurtle(t, turtle,
		"    sysml:declaredName \"A\" ;\n",
		"    sysml:declaredName \"A\" ;\n    sysml:isAbstract \"true\"^^xsd:boolean ;\n")
	want := strings.Replace(notation, "    part def A;", "    abstract part def A;", 1)
	if back := toNotation(t, edited); back != want {
		t.Errorf("the rebuilt member took the expression's line ending:\n--- want ---\n%q--- got ---\n%q", want, back)
	}
}

// A member introduced by `then` carries the word in its own text, whether its
// owner is written as it was or rebuilt around it.
func TestThenIsNotWrittenTwice(t *testing.T) {
	notation := `package P {
    action def Q {
        action a;
        // then b
        then action b;
    }
}
`
	turtle := idTurtle(t, notation)
	if back := toNotation(t, turtle); back != notation {
		t.Errorf("the notation changed:\n--- want ---\n%s--- got ---\n%s", notation, back)
	}
	edited := editTurtle(t, turtle,
		"    sysml:declaredName \"Q\" ;\n",
		"    sysml:declaredName \"Q\" ;\n    sysml:isAbstract \"true\"^^xsd:boolean ;\n")
	want := strings.Replace(notation, "    action def Q", "    abstract action def Q", 1)
	if back := toNotation(t, edited); back != want {
		t.Errorf("rebuilding the owner lost the members' text:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
}

// A positional succession is written as the `then` ahead of its target, so a
// target whose text does not state one the graph holds — or states one it no
// longer holds — is rebuilt, and the rest of the body is kept as written.
func TestPositionalSuccessionEditsRebuildTheirTarget(t *testing.T) {
	sequenced := `package P {
    action def Q {
        // first
        action a;
        // b follows
        then action b;
        // last
        action c;
    }
}
`
	turtle := idTurtle(t, sequenced)
	added := editTurtle(t, turtle, `        then action b;\n`, `        action b;\n`)
	want := strings.Replace(sequenced, "        // b follows\n", "", 1)
	if back := toNotation(t, added); back != want {
		t.Errorf("an added succession was not written:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
	if got, want := structural(t, idTurtle(t, toNotation(t, added))), structural(t, turtle); got != want {
		t.Errorf("the notation does not state the added succession:\n--- want ---\n%s--- got ---\n%s", want, got)
	}

	removed := withoutMember(t, turtle, "elmt:P__Q___402")
	want = strings.Replace(want, "        then action b;\n", "        action b;\n", 1)
	if back := toNotation(t, removed); back != want {
		t.Errorf("a removed succession was still written:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
	unnumbered := func(turtle []byte) string {
		return string(withoutTriples(t, []byte(structural(t, turtle)), "sysx:memberIndex"))
	}
	if got, want := unnumbered(idTurtle(t, toNotation(t, removed))), unnumbered(removed); got != want {
		t.Errorf("the notation still states the removed succession:\n--- want ---\n%s--- got ---\n%s", want, got)
	}
}

// withoutMember drops the given member of P from a Turtle document, its own members
// with it, from the owner's typed triples and JSON annotations alike; the rest keep their numbers.
func withoutMember(t *testing.T, turtle []byte, member string) []byte {
	t.Helper()
	id := strings.TrimPrefix(member, "elmt:")
	var kept []string
	for _, line := range strings.Split(string(turtle), "\n") {
		if strings.Contains(line, member) && !strings.HasPrefix(line, member) {
			for _, ref := range []string{member + "_om", member} {
				line = strings.ReplaceAll(line, ", "+ref, "")
				line = strings.ReplaceAll(line, ref+", ", "")
			}
		}
		if strings.HasPrefix(strings.TrimSpace(line), "json:") {
			for _, ref := range []string{id + "_om", id} {
				entry := `{\"@id\":\"` + ref + `\"}`
				line = strings.ReplaceAll(line, entry+",", "")
				line = strings.ReplaceAll(line, ","+entry, "")
			}
		}
		kept = append(kept, line)
	}
	var subjects []string
	for _, block := range strings.Split(strings.Join(kept, "\n"), "\n\n") {
		subject, _, _ := strings.Cut(block, "\n")
		if subject == member || strings.HasPrefix(subject, member+"_") {
			subjects = append(subjects, subject)
		}
	}
	return withoutSubjects(t, []byte(strings.Join(kept, "\n")), subjects...)
}

// withoutSubjects drops every block of a Turtle document describing one of the
// given subjects.
func withoutSubjects(t *testing.T, turtle []byte, subjects ...string) []byte {
	t.Helper()
	var kept []string
	for _, block := range strings.Split(string(turtle), "\n\n") {
		dropped := false
		for _, subject := range subjects {
			dropped = dropped || strings.HasPrefix(block, subject+"\n")
		}
		if !dropped {
			kept = append(kept, block)
		}
	}
	return []byte(strings.Join(kept, "\n\n"))
}

// Identity the text no longer carries is re-materialized as annotations; text
// that still carries them is kept as written.
func TestIdentityIsRematerializedIntoStaleText(t *testing.T) {
	const projectRef = `@IdentityMetadata::ProjectRef { projectId = "proj-1"; org = "acme"; }`
	const elementID = `@IdentityMetadata::ElementId { id = "shared"; }`
	src := "package P {\n    " + projectRef + "\n    part def A {\n        " + elementID + "\n    }\n    part a : A; // uses A\n}\n"
	turtle := idTurtle(t, src)
	if back := toNotation(t, turtle); back != src {
		t.Errorf("annotated notation changed:\n--- want ---\n%s--- got ---\n%s", src, back)
	}
	// The scope root's text loses its ProjectRef: the root is rebuilt with the
	// annotation, its members are kept as written.
	stale := editTurtle(t, turtle, "    "+strings.ReplaceAll(projectRef, `"`, `\"`)+`\n`, "")
	want := "package P {\n    " + projectRef + "\n    part def A {\n        " + elementID + "\n    }\n    part a : A; // uses A\n}\n"
	if back := toNotation(t, stale); back != want {
		t.Errorf("ProjectRef was not re-materialized:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
	// Both annotations gone from the text: everything but the untouched
	// member is rebuilt.
	stale = editTurtle(t, stale, "        "+strings.ReplaceAll(elementID, `"`, `\"`)+`\n`, "")
	if back := toNotation(t, stale); back != want {
		t.Errorf("ElementId was not re-materialized:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
	// Without any source text the same notation is built from the graph alone.
	if back := toNotation(t, withoutTriples(t, turtle, "sysx:sourceText")); back != strings.Replace(want, " // uses A", "", 1) {
		t.Errorf("stripped graph printed differently:\n%s", back)
	}
}

// An expression's text is checked like an element's: an edited expression graph
// wins over the text of the declaration holding it and over the node's own.
func TestEditedExpressionDoesNotResurrectStaleText(t *testing.T) {
	turtle := idTurtle(t, `package P {
    attribute mass : Real = 4; // the mass
    attribute count : Integer = 2; // and the count
}
`)
	// The literal's value is edited; the text of the literal and of the
	// attribute still say 4.
	turtle = editTurtle(t, turtle, `sysml:value "4"^^xsd:integer`, `sysml:value "5"^^xsd:integer`)
	back := toNotation(t, turtle)
	want := `package P {
    attribute mass : Real = 5;
    attribute count : Integer = 2; // and the count
}
`
	if back != want {
		t.Errorf("the edited expression did not win:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
}

// A string edited in the graph comes back as the literal the lexer reads to
// that value, whatever characters it holds.
func TestEditedStringValueIsWrittenAsALiteral(t *testing.T) {
	turtle := idTurtle(t, `package P {
    attribute label : String = "plain"; // the label
    attribute count : Integer = 2; // and the count
}
`)
	const value = "say \"hi\"\\\n\t\r\b\f"
	turtle = editTurtle(t, turtle, `sysml:value "plain"`, "sysml:value "+quoteLiteral(value))
	back := toNotation(t, turtle)
	want := `package P {
    attribute label : String = "say \"hi\"\\\n\t\r\b\f";
    attribute count : Integer = 2; // and the count
}
`
	if back != want {
		t.Errorf("the edited string did not win:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
	again := idTurtle(t, back)
	if !strings.Contains(string(again), quoteLiteral(value)) {
		t.Errorf("the notation does not state the edited value %q:\n%s", value, again)
	}
	if got, want := structural(t, again), structural(t, turtle); got != want {
		t.Errorf("the notation does not state the edited graph:\n--- want ---\n%s--- got ---\n%s", want, got)
	}
}

// quoteLiteral writes a value as the Turtle literal the writer emits.
func quoteLiteral(value string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "\t", `\t`, "\b", `\b`, "\f", `\f`)
	return `"` + r.Replace(value) + `"`
}

// structural is a Turtle document without its source text.
func structural(t *testing.T, turtle []byte) string {
	t.Helper()
	return string(withoutTriples(t, withoutTriples(t, turtle, "sysx:sourceText"), "sysx:sourceTail"))
}

// A member written on its owner's lines, such as an accept's payload, has no
// text of its own: an edit to it rebuilds the whole construct, and only that.
func TestEditedInlineMemberRebuildsItsConstruct(t *testing.T) {
	turtle := idTurtle(t, `// The drive, as modelled.
package Accepts {
    attribute def Cmd;
    attribute def Other;
    action def Drive {
        // waits for a command
        accept sig : Cmd;
        // then goes
        action go;
    }
}
`)
	if strings.Contains(string(turtle), `sysx:sourceText "        accept`) || strings.Contains(string(turtle), `sysx:sourceTail ";`) {
		t.Fatalf("the accept is split across its payload:\n%s", turtle)
	}
	turtle = editTurtle(t, turtle, "sysml:type elmt:Accepts__Cmd ;", "sysml:type elmt:Accepts__Other ;")
	back := toNotation(t, turtle)
	want := `// The drive, as modelled.
package Accepts {
    attribute def Cmd;
    attribute def Other;
    action def Drive {
        accept sig : Other;
        // then goes
        action go;
    }
}
`
	if back != want {
		t.Errorf("the edited payload did not rebuild its accept alone:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
	if got, want := structural(t, idTurtle(t, back)), structural(t, turtle); got != want {
		t.Errorf("the notation does not state the edited graph:\n--- want ---\n%s--- got ---\n%s", want, got)
	}
}

// A syntax error cannot be checked against the graph or pinned on one element,
// so the whole document falls back to canonical notation.
func TestUnparsableSourceTextIsIgnored(t *testing.T) {
	turtle := editTurtle(t, commentedTurtle(t), `part def Hub;\n`, `part def Hub\n`)
	back := toNotation(t, turtle)
	if want := toNotation(t, withoutTriples(t, turtle, "sysx:sourceText")); back != want {
		t.Errorf("unparsable text was not replaced:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
	if strings.Contains(back, "part def Hub\n") || !strings.Contains(back, "part def Hub;") {
		t.Errorf("the notation is not valid:\n%s", back)
	}
}

// A KerML document that also reads clean as SysML, with a different meaning:
// `binding [1] a = b` names the binding `a` there. The graph records the
// grammar its text was written in, so the check reads it as the file was.
const kermlBindings = `package Bindings {
    feature target[1];
    feature a[1];
    // an anonymous binding with a multiplicity
    binding [1] a = target;
}
`

func TestSourceTextIsReadInTheLanguageItWasWrittenIn(t *testing.T) {
	turtle, err := export.Convert("m.kerml", []byte(kermlBindings), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	if !strings.Contains(string(turtle), `sysx:sourceLanguage "kerml"`) {
		t.Fatalf("the graph does not record the language:\n%s", turtle)
	}
	if back := toNotation(t, turtle); back != kermlBindings {
		t.Errorf("round trip changed the notation:\n--- want ---\n%s--- got ---\n%s", kermlBindings, back)
	}
	// A root stripped of its own text still says what grammar its members' text is in.
	headless := editTurtle(t, turtle, `sysx:sourceText "package Bindings {\n" ;`, "")
	if back := toNotation(t, headless); !strings.Contains(back, "// an anonymous binding with a multiplicity\n    binding [1] a = target;") {
		t.Errorf("the members' text was not read as KerML:\n%s", back)
	}
	if !strings.Contains(string(idTurtle(t, commented)), `sysx:sourceLanguage "sysml"`) {
		t.Errorf("a SysML file does not record its language")
	}
}

// A buffer with no extension — standard input, a REPL session — is read as
// SysML with KerML's `all` prefix, so `part all : T;` is an anonymous part
// there and the part named `all` in a .sysml file. The graph records no
// language for it, and the check reads the text as such a buffer again.
func TestSourceTextOfAnExtensionlessBufferIsReadAsOne(t *testing.T) {
	src := "package P {\n    part def T;\n    // every T\n    part all : T;\n}\n"
	turtle, err := export.Convert("<stdin>", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	if strings.Contains(string(turtle), "sysx:sourceLanguage") {
		t.Fatalf("an extensionless buffer records a language:\n%s", turtle)
	}
	if strings.Contains(string(turtle), `sysml:declaredName "all"`) {
		t.Fatalf("`all` was read as a name:\n%s", turtle)
	}
	if back := toNotation(t, turtle); back != src {
		t.Errorf("round trip changed the notation:\n--- want ---\n%s--- got ---\n%s", src, back)
	}
	// Read as SysML instead, the same text names the part `all`: a different
	// model, so the text is not trusted, and no SysML declaration written in
	// its place states an anonymous part of every T either.
	sysml := editTurtle(t, turtle, `sysml:declaredName "P" ;`, `sysml:declaredName "P" ; sysx:sourceLanguage "sysml" ;`)
	refusedAsUnsupported(t, "m", sysml, "the reference to P::T from P::@1")
}

// Roots recording different languages cannot be read as one document, so their
// text is not trusted: the graph is written canonically.
func TestRootsOfTwoLanguagesAreWrittenCanonically(t *testing.T) {
	two := "// two\npackage A { part def P; }\npackage B { part def Q; }\n"
	turtle := idTurtle(t, two)
	if back := toNotation(t, turtle); back != two {
		t.Fatalf("round trip changed the notation:\n--- want ---\n%s--- got ---\n%s", two, back)
	}
	mixed := editTurtle(t, turtle, `sysx:sourceLanguage "sysml"`, `sysx:sourceLanguage "kerml"`)
	if back, want := toNotation(t, mixed), toNotation(t, withoutTriples(t, turtle, "sysx:sourceText")); back != want {
		t.Errorf("text of two languages was trusted:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
}
