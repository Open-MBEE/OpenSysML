package docplan

import (
	"strings"
	"testing"
)

// outline renders a content tree as one line per section, indented by depth,
// with the name and title of each, and the kind of every other block.
func outline(content []Content, depth int) string {
	var b strings.Builder
	for _, node := range content {
		b.WriteString(strings.Repeat("  ", depth))
		if node.Kind() == ContentSection {
			b.WriteString(node.Name() + " " + node.Title() + "\n")
			b.WriteString(outline(node.Children(), depth+1))
			continue
		}
		b.WriteString(string(node.Kind()) + "\n")
	}
	return b.String()
}

const wholeBodyModel = `
	part def Other :> Document {
		attribute redefines title = "Other";
		rep language "markdown" /* # Shared Notes
		 *
		 * See [](sysml:Report::overview) and [the budget](sysml:Report::mass_budget).
		 */
	}
	part def Report :> Document {
		attribute redefines title = "Report";
		part summary : Paragraph {
			:>> text = "Before the body.";
		}
		rep language "markdown" /* Lead; see [](sysml:mass_budget), [](sysml:mass_budget_2),
		 * [](sysml:Other::shared_notes), [](sysml:appendix) and [](sysml:inside).
		 *
		 * # Overview
		 *
		 * Under the overview.
		 *
		 * ## Mass budget
		 *
		 * - one
		 *
		 * Mass Budget
		 * -----------
		 *
		 * ### 3rd level, deep!
		 *
		 * # Design
		 *
		 * # appendix
		 */
		part appendix : Section {
			:>> title = "Appendix";
			rep language "markdown" /* ## Inside
			 *
			 * See [](sysml:overview).
			 */
		}
	}
`

// A document body's headings open sections nested by level among the sibling
// parts in declaration order, named by a slug of the title made unique
// against the document's declared names and earlier headings.
func TestCompileWholeDocumentBody(t *testing.T) {
	fixture := loadPlanningFixture(t, wholeBodyModel)
	plan := fixture.mustCompile(t, "Report")
	want := strings.TrimLeft(`
paragraph
paragraph
overview Overview
  paragraph
  mass_budget Mass budget
    list
  mass_budget_2 Mass Budget
    _3rd_level_deep 3rd level, deep!
design Design
appendix_2 appendix
appendix Appendix
  inside Inside
    paragraph
`, "\n")
	if got := outline(plan.Content(), 0); got != want {
		t.Fatalf("outline:\n%s\nwant:\n%s", got, want)
	}
	lead := plan.Content()[1]
	var refs []string
	for _, run := range lead.Runs() {
		if run.Kind() == RunRef {
			refs = append(refs, run.RefDocument()+":"+strings.Join(run.RefPath(), "/")+"="+run.Text())
		}
	}
	wantRefs := []string{
		":overview/mass_budget=Mass budget",
		":overview/mass_budget_2=Mass Budget",
		"Observatory::Other:shared_notes=Shared Notes",
		":appendix=Appendix",
		":appendix/inside=Inside",
	}
	if strings.Join(refs, "\n") != strings.Join(wantRefs, "\n") {
		t.Errorf("references:\n%s\nwant:\n%s", strings.Join(refs, "\n"), strings.Join(wantRefs, "\n"))
	}
	for _, got := range plan.Content()[5].Children()[0].Children()[0].Runs() {
		if got.Kind() == RunRef && (strings.Join(got.RefPath(), "/") != "overview" || got.RefDocument() != "") {
			t.Errorf("section body reference = %+v", got)
		}
	}

	other := fixture.mustCompile(t, "Other")
	var cross []string
	for _, run := range other.Content()[0].Children()[0].Runs() {
		if run.Kind() == RunRef {
			cross = append(cross, run.RefDocument()+":"+strings.Join(run.RefPath(), "/")+"="+run.Text())
		}
	}
	if got, want := strings.Join(cross, "\n"), "Observatory::Report:overview=Overview\nObservatory::Report:overview/mass_budget=the budget"; got != want {
		t.Errorf("cross-document references:\n%s\nwant:\n%s", got, want)
	}
}

// A heading in a paragraph body is not a paragraph, and a section heading
// deeper than its section allows is out of sequence from the section's base.
func TestCompileWholeBodyRefusals(t *testing.T) {
	for name, tc := range map[string]struct {
		body     string
		kind     ErrorKind
		contains string
	}{
		"paragraph": {
			body:     "part p : Paragraph {\n rep language \"markdown\" /* # Heading\n *\n * text */\n }",
			kind:     ErrorMarkupBodyNotParagraph,
			contains: "yields a section rather than exactly one paragraph",
		},
		"second body skips a level": {
			body:     "rep language \"markdown\" /* # One */\n rep language \"markdown\" /* ## Two\n *\n * #### Four */",
			kind:     ErrorMarkupHeadingLevel,
			contains: `level 4 heading "Four" is out of sequence: expected a level between 2 and 3 (body line 3)`,
		},
		"unknown name": {
			body:     "rep language \"markdown\" /* # One\n *\n * See [](sysml:two). */",
			kind:     ErrorUnknownRefTarget,
			contains: "names unknown target two (body line 3)",
		},
	} {
		t.Run(name, func(t *testing.T) {
			fixture := loadPlanningFixture(t, `
				part def Report :> Document {
					attribute redefines title = "Report";
					`+tc.body+`
				}
			`)
			_, err := fixture.compile(t, "Report")
			planning := bodyPlanningError(t, err)
			if planning.Kind != tc.kind || !strings.Contains(err.Error(), tc.contains) {
				t.Fatalf("got %s: %v\nwant %s containing %q", planning.Kind, err, tc.kind, tc.contains)
			}
		})
	}
}

// An AsciiDoc body's section titles open sections the same way, an explicit
// `[[id]]` or `[#id]` anchor naming the section, and an xref spelling that
// anchor resolves to it.
func TestCompileWholeDocumentAsciiDocBody(t *testing.T) {
	useRecordedAsciiDoc(t)
	model := `
	part def Report :> Document {
		attribute redefines title = "Report";
		` + asciiDocBody(t, "asciidoc_sections.adoc") + `
		part closing : Paragraph {
			rep language "markdown" /* See <<mass_budget>> as [](sysml:mass_budget) and [](sysml:design). */
		}
	}
`
	fixture := loadPlanningFixture(t, model)
	plan := fixture.mustCompile(t, "Report")
	want := strings.TrimLeft(`
paragraph
overview Overview
  paragraph
  mass_budget Mass budget
    paragraph
design Design of the sysml tool
  list
paragraph
`, "\n")
	if got := outline(plan.Content(), 0); got != want {
		t.Fatalf("outline:\n%s\nwant:\n%s", got, want)
	}
	var refs []string
	for _, run := range plan.Content()[3].Runs() {
		if run.Kind() == RunRef {
			refs = append(refs, strings.Join(run.RefPath(), "/")+"="+run.Text())
		}
	}
	if got, want := strings.Join(refs, " "), "overview/mass_budget=Mass budget design=Design of the sysml tool"; got != want {
		t.Errorf("references %q, want %q", got, want)
	}
}
