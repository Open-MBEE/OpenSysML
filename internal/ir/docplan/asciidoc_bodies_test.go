package docplan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan/markup"
	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan/markup/markuptest"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

const asciiDocRecordings = "markup/testdata"

// useRecordedAsciiDoc points the AsciiDoc parser at the recorded converter.
func useRecordedAsciiDoc(t *testing.T) {
	t.Helper()
	converter, err := markuptest.RecordedConverter(t.TempDir(), asciiDocRecordings)
	if err != nil {
		t.Skip(err)
	}
	t.Setenv(markup.AsciiDocEnv, converter)
}

// asciiDocBody writes the recorded body as a textual representation.
func asciiDocBody(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(asciiDocRecordings, name))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	for i := 1; i < len(lines); i++ {
		if lines[i] == "" {
			lines[i] = " *"
		} else {
			lines[i] = " * " + lines[i]
		}
	}
	return `rep language "asciidoc" /* ` + strings.Join(lines, "\n") + "\n */"
}

const asciiDocModel = `
	package Optics {
		part mirror;
		part mount;
	}
	part def Report :> Document {
		attribute redefines title = "Report";
		%s
		part summary : Paragraph {
			:>> text = "Summary.";
		}
		part notes : Paragraph {
			%s
		}
	}
`

// An AsciiDoc body expands into every content block it maps onto: paragraphs
// (an admonition among them), lists, a static table with its header, widths
// and rows, a formula and an image; a paragraph body yields that paragraph.
func TestCompileDocumentAsciiDocBody(t *testing.T) {
	useRecordedAsciiDoc(t)
	model := strings.Replace(strings.Replace(asciiDocModel, "%s", asciiDocBody(t, "asciidoc_body.adoc"), 1), "%s", asciiDocBody(t, "asciidoc_paragraph.adoc"), 1)
	fixture := loadPlanningFixture(t, model)
	full := "package Observatory {" + planningImports + model + "}"
	plan := fixture.mustCompile(t, "Report")
	content := plan.Content()
	kinds := make([]string, len(content))
	for i, node := range content {
		kinds[i] = string(node.Kind())
	}
	if got, want := strings.Join(kinds, " "), "paragraph list paragraph list table formula image paragraph paragraph"; got != want {
		t.Fatalf("content kinds %q, want %q", got, want)
	}
	runs := content[0].Runs()
	if got := runTexts(runs); len(got) != 21 || got[1] != "em" || got[11] != "mirror" || got[13] != "the mount" || got[17] != "notes" {
		t.Errorf("paragraph runs %q", got)
	}
	if runs[11].Kind() != RunRef || runs[11].RefElement() == "" || runs[15].Kind() != RunRef || runs[15].Text() != "the summary" ||
		runs[19].Style() != StyleMath || runs[7].Kind() != RunLink || runs[7].Target() != "https://x.io/a" {
		t.Errorf("paragraph runs %+v", runs)
	}
	if note := runTexts(content[2].Runs()); strings.Join(note, "|") != "Note:| Mind the |gap|." || content[2].Runs()[0].Style() != StyleStrong {
		t.Errorf("admonition runs %q", note)
	}
	table := content[4]
	if table.Query() != nil || table.Caption() != "Masses" || strings.Join(table.ColumnLabels(), "|") != "Part|Mass" ||
		len(table.ColumnWidths()) != 2 || table.ColumnWidths()[0] != 3 {
		t.Errorf("table %+v", table)
	}
	rows := table.Rows()
	if len(rows) != 2 || strings.Join(rows[0].Cells(), "|") != "Mirror|12 kg" || strings.Join(rows[1].Cells(), "|") != "Mount|4 kg" ||
		rows[1].Origin().Doc == "" {
		t.Errorf("table rows %+v", rows)
	}
	if formula := content[5]; formula.Source() != `m \propto D^2` || formula.Caption() != "Scaling" {
		t.Errorf("formula %+v", formula)
	}
	if image := content[6]; image.Location() != "https://example.test/figures/optics.png" || image.Alt() != "The optics" || image.Caption() != "Optics" {
		t.Errorf("image %+v", image)
	}
	if notes := content[8]; notes.Name() != "notes" || runTexts(notes.Runs())[1] != "named" || notes.Runs()[3].Kind() != RunRef {
		t.Errorf("paragraph body %+v", notes)
	}
	if o := content[4].Origin(); !o.Located() || !strings.HasPrefix(full[o.Span.Offset:o.Span.End()], ".Masses") || !strings.HasSuffix(full[o.Span.Offset:o.Span.End()], "|===") {
		t.Errorf("table origin reads %q", full[o.Span.Offset:o.Span.End()])
	}
	if o := content[0].Runs()[1].Origin(); full[o.Span.Offset:o.Span.End()] != "em" {
		t.Errorf("emphasis origin reads %q", full[o.Span.Offset:o.Span.End()])
	}
}

// Without the converter the body is a typed planning error naming the
// provisioning script and the variable; a converter that fails is another.
func TestCompileAsciiDocBodyToolErrors(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part def Report :> Document {
			attribute redefines title = "Report";
			rep language "AsciiDoc" /* A body. */
		}
	`)
	t.Setenv(markup.AsciiDocEnv, filepath.Join(t.TempDir(), "missing-converter"))
	_, err := fixture.compile(t, "Report")
	planning := bodyPlanningError(t, err)
	if planning.Kind != ErrorMarkupToolMissing {
		t.Fatalf("kind %s, want %s: %v", planning.Kind, ErrorMarkupToolMissing, err)
	}
	for _, want := range []string{"asciidoc body", "download-doc-pdf-toolchain.sh", markup.AsciiDocEnv} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%v lacks %q", err, want)
		}
	}

	useRecordedAsciiDoc(t)
	_, err = fixture.compile(t, "Report")
	planning = bodyPlanningError(t, err)
	if planning.Kind != ErrorMarkupToolFailed || !strings.Contains(err.Error(), "no recorded converter answer") {
		t.Errorf("kind %s: %v, want %s naming the converter's message", planning.Kind, err, ErrorMarkupToolFailed)
	}
}

func TestValidateLeavesBodiesOfMissingConverterUnchecked(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part def Report :> Document {
			attribute redefines title = "Report";
			rep language "AsciiDoc" /* A body. */
			part intro : Paragraph {
				rep language "AsciiDoc" /* Another body. */
			}
			part body : Section {
				attribute redefines title = "Body";
				rep language "AsciiDoc" /* A third body. */
			}
			part empty : Paragraph;
		}
	`)
	t.Setenv(markup.AsciiDocEnv, filepath.Join(t.TempDir(), "missing-converter"))

	// Validation goes on past the bodies, one warning for all three, and
	// still finds the paragraph that has no text.
	warnings, err := Validate(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, "Report"))
	if planning := planningError(t, err); planning.Kind != ErrorMissingText || planning.Content != "Observatory::Report::empty" {
		t.Errorf("error %v, want %s at Observatory::Report::empty", err, ErrorMissingText)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings %v, want one", warnings)
	}
	warning := warnings[0]
	if warning.Kind != ErrorMarkupToolMissing || warning.Kind.Severity() != diag.SeverityWarning || warning.Bodies != 3 || warning.Content != "Observatory::Report" {
		t.Errorf("warning %+v, want %s (a warning) over 3 bodies, the first in Observatory::Report", warning, ErrorMarkupToolMissing)
	}
	for _, want := range []string{"carries 3 asciidoc bodies that were not checked, the first in content Observatory::Report", "missing-converter", "(" + markup.AsciiDocEnv + " names it)", "download-doc-pdf-toolchain.sh"} {
		if !strings.Contains(warning.Error(), want) {
			t.Errorf("%v lacks %q", warning, want)
		}
	}
	if ErrorMarkupToolFailed.Severity() != diag.SeverityError {
		t.Errorf("%s is a %s, want an error", ErrorMarkupToolFailed, ErrorMarkupToolFailed.Severity())
	}

	// A rendering needs the bodies: Compile stops at the first.
	_, err = fixture.compile(t, "Report")
	if planning := bodyPlanningError(t, err); planning.Kind != ErrorMarkupToolMissing || planning.Bodies != 0 || !strings.Contains(err.Error(), "could not be parsed") {
		t.Errorf("Compile: %v, want %s stopping at the body", err, ErrorMarkupToolMissing)
	}
}

func TestValidateReportsOneWarningPerBody(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part def Report :> Document {
			attribute redefines title = "Report";
			part intro : Paragraph {
				rep language "AsciiDoc" /* A body. */
			}
		}
	`)
	t.Setenv(markup.AsciiDocEnv, filepath.Join(t.TempDir(), "missing-converter"))
	warnings, err := Validate(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, "Report"))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(warnings) != 1 || warnings[0].Bodies != 1 || !strings.Contains(warnings[0].Error(), "content Observatory::Report::intro carries a asciidoc body that was not checked") {
		t.Errorf("warnings %v, want one over the paragraph's body", warnings)
	}
}
