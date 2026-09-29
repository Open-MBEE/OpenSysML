package edit

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestAddCommentAtTheRoot(t *testing.T) {
	m := loadContent(t, "c.sysml", "package P;\n")
	got := applyContent(t, m, AddComment("", "Top level."))
	if want := "package P;\ncomment /* Top level.*/\n"; got != want {
		t.Fatalf("content =\n%s\nwant\n%s", got, want)
	}
}

func TestAddCommentInABodyWithNameAboutAndLocale(t *testing.T) {
	m := loadContent(t, "c.sysml", "package P {\n    part def A;\n    part def B;\n}\n")
	op := AddComment("P", "Line one.\nLine two.")
	op.DocName = "Note"
	op.About = []string{"A", "P::'B'"}
	op.DocLocale = "en"
	got := applyContent(t, m, op)
	want := "package P {\n    part def A;\n    part def B;\n" +
		"    comment Note about A, P::'B' locale \"en\" /* Line one.\n     * Line two.*/\n}\n"
	if got != want {
		t.Fatalf("content =\n%s\nwant\n%s", got, want)
	}
}

func TestAddCommentOpensABodylessOwner(t *testing.T) {
	m := loadContent(t, "c.sysml", "package P {\n    part def A;\n}\n")
	op := AddComment("P::A", "About A.")
	op.About = []string{"A"}
	got := applyContent(t, m, op)
	if want := "    part def A {\n        comment about A /* About A.*/\n    }\n"; !strings.Contains(got, want) {
		t.Fatalf("content does not contain %q:\n%s", want, got)
	}
}

func TestAddCommentInKerML(t *testing.T) {
	m := loadContent(t, "c.kerml", "package P {\n    classifier A;\n}\n")
	got := applyContent(t, m, AddComment("P", "In KerML."))
	if want := "    classifier A;\n    comment /* In KerML.*/\n}\n"; !strings.Contains(got, want) {
		t.Fatalf("content does not contain %q:\n%s", want, got)
	}
}

func TestAddCommentBodyReadsBackExactly(t *testing.T) {
	for _, body := range []string{
		"One line.", "", "  ", " leading", "trailing ", "a \n  indented\n\ttabbed\t",
		"\nopens with a blank line", "ends with a line break\n", "* bullet\n* bullet", "/ slash // and /* opener",
	} {
		m := loadContent(t, "c.sysml", "package P {\n    part def A;\n}\n")
		got := applyContent(t, m, AddComment("P::A", body))
		sf := source.New("c.sysml", []byte(got))
		p := parser.New(sf)
		root := p.ParseFile()
		if len(p.Diagnostics) > 0 {
			t.Fatalf("result does not parse: %v\n%s", p.Diagnostics, got)
		}
		var read []string
		ast.Inspect(root, func(n ast.Node) bool {
			if c, ok := n.(*ast.Comment); ok {
				read = append(read, source.CommentBody(string(sf.Bytes()[c.BodySpan.Offset:c.BodySpan.End()])))
			}
			return true
		})
		if len(read) != 1 || read[0] != body {
			t.Fatalf("comment read back as %q, want %q:\n%s", read, body, got)
		}
	}
}

func TestAddCommentRefusals(t *testing.T) {
	m := loadContent(t, "c.sysml", "package P {\n    part def A;\n}\n")
	named := func(name string) Operation {
		op := AddComment("P", "Named.")
		op.DocName = name
		return op
	}
	about := func(ref string) Operation {
		op := AddComment("P", "About.")
		op.About = []string{ref}
		return op
	}
	for _, tc := range []struct {
		name string
		op   Operation
		want Failure
	}{
		{"unknown owner", AddComment("P::Missing", "Text."), FailureOwnerUnknown},
		{"comment close", AddComment("P", "a */ b"), FailureInvalidValue},
		{"carriage return", AddComment("P", "a\rb"), FailureInvalidValue},
		{"name taken", named("A"), FailureMemberNameTaken},
		{"name keyword", named("part"), FailureInvalidName},
		{"about not a name", about("A B"), FailureInvalidName},
		{"about a feature chain", about("A.x"), FailureInvalidName},
		{"about unresolved", about("Missing"), FailureResultInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			addFailure(t, m, tc.op, tc.want)
		})
	}
}

func TestAddNoteAboveAMember(t *testing.T) {
	m := loadContent(t, "n.sysml", "package P {\n    attribute def A;\n    // kept\n    attribute def B;\n}\n")
	got := applyContent(t, m, AddNote("P::A", "DimensionOneValue"), AddNote("P::B", "second"))
	want := "package P {\n    // DimensionOneValue\n    attribute def A;\n    // kept\n    // second\n    attribute def B;\n}\n"
	if got != want {
		t.Fatalf("content =\n%s\nwant\n%s", got, want)
	}
}

func TestAddNoteBeforeAnInlineMember(t *testing.T) {
	m := loadContent(t, "n.sysml", "package P {\n    part def A { attribute x; attribute y; }\n}\n")
	got := applyContent(t, m, AddNote("P::A::y", "why"), AddNote("P::A::x", ""))
	want := "package P {\n    part def A {\n        //\n        attribute x;\n        // why\n        attribute y; }\n}\n"
	if got != want {
		t.Fatalf("content =\n%s\nwant\n%s", got, want)
	}
}

func TestAddNoteAtTheRootAndOnPrefixedMembers(t *testing.T) {
	m := loadContent(t, "n.sysml", "package P {\n    private part def A;\n    part def B {\n        in attribute x;\n    }\n}\n")
	got := applyContent(t, m, AddNote("P", "top"), AddNote("P::A", "private"), AddNote("P::B::x", "input"))
	want := "// top\npackage P {\n    // private\n    private part def A;\n    part def B {\n" +
		"        // input\n        in attribute x;\n    }\n}\n"
	if got != want {
		t.Fatalf("content =\n%s\nwant\n%s", got, want)
	}
}

func TestAddNoteSurvivesReparsingAndLaterEdits(t *testing.T) {
	m := loadContent(t, "n.sysml", "package P {\n    attribute def A;\n    part def B {\n        attribute a : A;\n    }\n}\n")
	noted := applyContent(t, m, AddNote("P::B::a", "DimensionOneValue"))
	m = loadContent(t, "n.sysml", noted)
	requireClean(t, m)
	add := AddMember("P::B", "attribute", "b")
	add.Type = "A"
	got := applyContent(t, m, Rename("P::B::a", "c"), add, AddDocumentation("P::B", "Documented."))
	want := "package P {\n    attribute def A;\n    part def B {\n        doc /* Documented.*/\n" +
		"        // DimensionOneValue\n        attribute c : A;\n        attribute b : A;\n    }\n}\n"
	if got != want {
		t.Fatalf("content =\n%s\nwant\n%s", got, want)
	}
	m = loadContent(t, "n.sysml", got)
	moved := applyContent(t, m, Move("P::B::c", "P"))
	if !strings.Contains(moved, "    // DimensionOneValue\n    attribute c : A;\n") {
		t.Fatalf("the note did not move with its member:\n%s", moved)
	}
	m = loadContent(t, "n.sysml", got)
	if deleted := applyContent(t, m, Delete("P::B::c", false)); strings.Contains(deleted, "DimensionOneValue") {
		t.Fatalf("the note outlived its member:\n%s", deleted)
	}
}

func TestAddNoteRefusals(t *testing.T) {
	m := loadContent(t, "n.sysml", "package P {\n    part def A;\n}\n")
	for _, tc := range []struct {
		name string
		op   Operation
		want Failure
	}{
		{"newline", AddNote("P::A", "one\ntwo"), FailureInvalidValue},
		{"carriage return", AddNote("P::A", "one\rtwo"), FailureInvalidValue},
		{"unknown target", AddNote("P::Missing", "text"), FailureUnknownTarget},
		{"library target", AddNote("ISQ::DurationValue", "text"), FailureUnknownTarget},
	} {
		t.Run(tc.name, func(t *testing.T) {
			addFailure(t, m, tc.op, tc.want)
		})
	}
}
