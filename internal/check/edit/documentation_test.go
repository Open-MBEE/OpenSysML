package edit

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func applyContent(t *testing.T, m Model, ops ...Operation) string {
	t.Helper()
	res, err := Apply(m, ops)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	return string(res.Content)
}

func TestAddMemberWritesDocumentationBody(t *testing.T) {
	m := loadContent(t, "doc.sysml", "package P {\n    item def Bread;\n}\n")
	requireClean(t, m)
	add := AddMember("P", "action def", "ToastBread")
	add.Doc = "Transform bread into toast."
	param := AddMember("P::ToastBread", "ref", "bread")
	param.Direction = "in"
	param.Type = "Bread"
	got := applyContent(t, m, add, param)
	want := "package P {\n    item def Bread;\n    action def ToastBread {\n" +
		"        doc /* Transform bread into toast.*/\n        in ref bread : Bread;\n    }\n}\n"
	if got != want {
		t.Fatalf("content =\n%s\nwant\n%s", got, want)
	}
}

func TestAddMemberWritesMultilineDocumentationUnderOwnerIndent(t *testing.T) {
	m := loadContent(t, "doc.sysml", "package P {\n    action def A;\n}\n")
	add := AddMember("P::A", "ref", "duration")
	add.Direction = "in"
	add.Type = "ISQ::DurationValue"
	add.Multiplicity = "[0..*]"
	add.Doc = "First line.\nSecond line.\n\nAfter a blank."
	got := applyContent(t, m, add)
	want := "package P {\n    action def A {\n        in ref duration : ISQ::DurationValue [0..*] {\n" +
		"            doc /* First line.\n             * Second line.\n             *\n" +
		"             * After a blank.*/\n        }\n    }\n}\n"
	if got != want {
		t.Fatalf("content =\n%s\nwant\n%s", got, want)
	}
}

func TestAddMemberWritesDocumentationWithTabs(t *testing.T) {
	m := loadContent(t, "doc.sysml", "package P {\n\tpart def A;\n}\n")
	add := AddMember("P", "part def", "B")
	add.Doc = "One.\nTwo."
	got := applyContent(t, m, add)
	if want := "\tpart def B {\n\t\tdoc /* One.\n\t\t * Two.*/\n\t}\n"; !strings.Contains(got, want) {
		t.Fatalf("content does not contain %q:\n%s", want, got)
	}
}

func TestAddDocumentationOpensBodyOfBodylessDeclaration(t *testing.T) {
	m := loadContent(t, "doc.sysml", "package P {\n    item def Bread; // kept\n    item def Toast;\n}\n")
	got := applyContent(t, m, AddDocumentation("P::Bread", "Sliced bread."))
	want := "package P {\n    item def Bread {\n        doc /* Sliced bread.*/\n    } // kept\n    item def Toast;\n}\n"
	if got != want {
		t.Fatalf("content =\n%s\nwant\n%s", got, want)
	}
}

func TestAddDocumentationPrecedesExistingMembers(t *testing.T) {
	m := loadContent(t, "doc.sysml",
		"package P {\n    part def A {\n        // leading note\n        attribute x;\n        attribute y;\n    }\n}\n")
	got := applyContent(t, m, AddDocumentation("P::A", "Documented."))
	want := "package P {\n    part def A {\n        doc /* Documented.*/\n        // leading note\n" +
		"        attribute x;\n        attribute y;\n    }\n}\n"
	if got != want {
		t.Fatalf("content =\n%s\nwant\n%s", got, want)
	}
}

func TestAddDocumentationIntoEmptyAndInlineBodies(t *testing.T) {
	m := loadContent(t, "doc.sysml", "package P {\n    part def A {\n    }\n    part def B { attribute x; }\n}\n")
	got := applyContent(t, m, AddDocumentation("P::A", "Empty."), AddDocumentation("P::B", "Inline."))
	for _, want := range []string{
		"    part def A {\n        doc /* Empty.*/\n    }\n",
		"    part def B { doc /* Inline.*/ attribute x; }\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("content does not contain %q:\n%s", want, got)
		}
	}
}

func TestAddDocumentationToPackageAndConnectorLikeUsages(t *testing.T) {
	m := loadContent(t, "doc.sysml", "package P {\n    requirement def R;\n    requirement r : R;\n"+
		"    part def S {\n        part a;\n        part b;\n        connect a to b;\n        satisfy r by a;\n    }\n"+
		"    state def M {\n        state idle;\n        state busy;\n        transition go first idle then busy;\n    }\n}\n")
	requireClean(t, m)
	got := applyContent(t, m,
		AddDocumentation("P", "The package."),
		AddDocumentation("P::M::go", "Starts work."),
	)
	for _, want := range []string{
		"package P {\n    doc /* The package.*/\n    requirement def R;\n",
		"        transition go first idle then busy {\n            doc /* Starts work.*/\n        }\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("content does not contain %q:\n%s", want, got)
		}
	}
}

func TestAddDocumentationToRelationshipDeclarations(t *testing.T) {
	for _, tc := range []struct {
		name, file, content, target, want string
	}{
		{"dependency", "doc.sysml",
			"package P {\n    part def A;\n    part def B;\n    dependency D from A to B;\n}\n", "P::D",
			"    dependency D from A to B {\n        doc /* Relates.*/\n    }\n"},
		{"dependency with body", "doc.sysml",
			"package P {\n    part def A;\n    part def B;\n    dependency D from A to B {\n        comment /* Kept. */\n    }\n}\n",
			"P::D",
			"    dependency D from A to B {\n        doc /* Relates.*/\n        comment /* Kept. */\n    }\n"},
		{"multiplicity", "doc.kerml",
			"package P {\n    multiplicity m [1..2];\n}\n", "P::m",
			"    multiplicity m [1..2] {\n        doc /* Relates.*/\n    }\n"},
		{"multiplicity with body", "doc.kerml",
			"package P {\n    multiplicity m [1..2] {\n        comment /* Kept. */\n    }\n}\n", "P::m",
			"    multiplicity m [1..2] {\n        doc /* Relates.*/\n        comment /* Kept. */\n    }\n"},
		{"relationship", "doc.kerml",
			"package P {\n    classifier A;\n    classifier B;\n    specialization S subclassifier A specializes B;\n}\n", "P::S",
			"    specialization S subclassifier A specializes B {\n        doc /* Relates.*/\n    }\n"},
		{"relationship with body", "doc.kerml",
			"package P {\n    classifier A;\n    classifier B;\n    specialization S subclassifier A specializes B {\n" +
				"        comment /* Kept. */\n    }\n}\n", "P::S",
			"    specialization S subclassifier A specializes B {\n        doc /* Relates.*/\n        comment /* Kept. */\n    }\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := loadContent(t, tc.file, tc.content)
			requireClean(t, m)
			got := applyContent(t, m, AddDocumentation(tc.target, "Relates."))
			if !strings.Contains(got, tc.want) {
				t.Fatalf("content does not contain %q:\n%s", tc.want, got)
			}
		})
	}
}

func TestAddDocumentationWritesNameAndLocale(t *testing.T) {
	m := loadContent(t, "doc.sysml", "package P {\n    part def A;\n}\n")
	op := AddDocumentation("P::A", "Anglais.")
	op.DocName = "English"
	op.DocLocale = "en-US"
	got := applyContent(t, m, op)
	if want := `doc English locale "en-US" /* Anglais.*/`; !strings.Contains(got, want) {
		t.Fatalf("content does not contain %q:\n%s", want, got)
	}
}

func TestAddDocumentationRefusesExistingDocumentation(t *testing.T) {
	m := loadContent(t, "doc.sysml", "package P {\n    part def A {\n        doc /* Old. */\n    }\n}\n")
	addFailure(t, m, AddDocumentation("P::A", "New."), FailureMemberNameTaken)
}

func TestAddDocumentationReplacesTheOneDocumentation(t *testing.T) {
	m := loadContent(t, "doc.sysml",
		"package P {\n    part def A {\n        doc Old /* Old. */\n        attribute x;\n    }\n}\n")
	op := AddDocumentation("P::A", "New.\nLines.")
	op.ReplaceDoc = true
	op.DocName = "Old"
	got := applyContent(t, m, op)
	want := "package P {\n    part def A {\n        doc Old /* New.\n         * Lines.*/\n        attribute x;\n    }\n}\n"
	if got != want {
		t.Fatalf("content =\n%s\nwant\n%s", got, want)
	}
}

func TestAddDocumentationReplaceKeepsVisibility(t *testing.T) {
	for _, vis := range []string{"", "public ", "private ", "protected "} {
		t.Run("visibility="+vis, func(t *testing.T) {
			m := loadContent(t, "doc.sysml",
				"package P {\n    part def A {\n        "+vis+"doc Summary /* Old. */\n    }\n}\n")
			requireClean(t, m)
			op := AddDocumentation("P::A", "New.")
			op.ReplaceDoc = true
			op.DocName = "Summary"
			got := applyContent(t, m, op)
			want := "package P {\n    part def A {\n        " + vis + "doc Summary /* New.*/\n    }\n}\n"
			if got != want {
				t.Fatalf("content =\n%s\nwant\n%s", got, want)
			}
		})
	}
}

func TestAddDocumentationReplaceRefusesAmbiguity(t *testing.T) {
	m := loadContent(t, "doc.sysml",
		"package P {\n    part def A {\n        doc /* One. */\n        doc /* Two. */\n    }\n}\n")
	op := AddDocumentation("P::A", "New.")
	op.ReplaceDoc = true
	addFailure(t, m, op, FailureAmbiguousTarget)
}

func TestAddDocumentationRefusals(t *testing.T) {
	m := loadContent(t, "doc.sysml", "package P {\n    part def A {\n        attribute x;\n    }\n}\n")
	named := AddDocumentation("P::A", "Named.")
	named.DocName = "x"
	badName := AddDocumentation("P::A", "Named.")
	badName.DocName = "part"
	docMember := AddMember("P", "part def", "B")
	docMember.Doc = "closes */ early"
	for _, tc := range []struct {
		name string
		op   Operation
		want Failure
	}{
		{"unknown target", AddDocumentation("P::Missing", "Text."), FailureUnknownTarget},
		{"comment close", AddDocumentation("P::A", "a */ b"), FailureInvalidValue},
		{"carriage return", AddDocumentation("P::A", "a\r\nb"), FailureInvalidValue},
		{"name taken", named, FailureMemberNameTaken},
		{"name keyword", badName, FailureInvalidName},
		{"member doc comment close", docMember, FailureInvalidValue},
	} {
		t.Run(tc.name, func(t *testing.T) {
			addFailure(t, m, tc.op, tc.want)
		})
	}
}

func TestAddDocumentationRefusesElementWithoutBody(t *testing.T) {
	m := loadContent(t, "doc.sysml", "package P {\n    part def A {\n        doc D /* Doc. */\n    }\n}\n")
	addFailure(t, m, AddDocumentation("P::A::D", "Doc of doc."), FailureOwnerNotNamespace)
}

func TestAddDocumentationRefusesLibraryElement(t *testing.T) {
	m := loadContent(t, "doc.sysml", "package P {\n    part def A;\n}\n")
	addFailure(t, m, AddDocumentation("ISQ::DurationValue", "Library."), FailureUnknownTarget)
}

func TestAddDocumentationBodyReadsBackExactly(t *testing.T) {
	for _, body := range []string{
		"One line.",
		"Signal from a control function: how long to apply heat.\nNo control function is modeled in this chapter, so this input\nis declared and typed but not yet connected to a value.",
		"Paragraph.\n\nAnother, with * inside and a trailing star *",
		"Unicode — ✓ and a / slash",
		"",
		"  ",
		" leading space",
		"trailing space ",
		"a \n  indented\n\ttabbed\t",
		"\nopens with a blank line",
		"ends with a line break\n",
		"* a bullet\n* another",
		"ends with a star*",
	} {
		m := loadContent(t, "doc.sysml", "package P {\n    part def A;\n}\n")
		got := applyContent(t, m, AddDocumentation("P::A", body))
		sf := source.New("doc.sysml", []byte(got))
		p := parser.New(sf)
		root := p.ParseFile()
		if len(p.Diagnostics) > 0 {
			t.Fatalf("result does not parse: %v\n%s", p.Diagnostics, got)
		}
		var read []string
		ast.Inspect(root, func(n ast.Node) bool {
			if d, ok := n.(*ast.Documentation); ok {
				read = append(read, source.CommentBody(string(sf.Bytes()[d.BodySpan.Offset:d.BodySpan.End()])))
			}
			return true
		})
		if len(read) != 1 || read[0] != body {
			t.Fatalf("documentation read back as %q, want %q:\n%s", read, body, got)
		}
	}
}
