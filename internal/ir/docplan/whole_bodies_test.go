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

// An explicit anchor is the section's name as written when it is a basic
// name, and the slug of the id otherwise; an xref spelt as the id, however
// it is written, resolves to the section.
func TestCompileWholeDocumentAsciiDocExplicitIDs(t *testing.T) {
	useRecordedAsciiDoc(t)
	fixture := loadPlanningFixture(t, `
	part def Report :> Document {
		attribute redefines title = "Report";
		`+asciiDocBody(t, "asciidoc_anchors.adoc")+`
	}
`)
	plan := fixture.mustCompile(t, "Report")
	want := strings.TrimLeft(`
MassBudget Mass budget
  paragraph
power_budget Power budget
  paragraph
`, "\n")
	if got := outline(plan.Content(), 0); got != want {
		t.Fatalf("outline:\n%s\nwant:\n%s", got, want)
	}
	var refs []string
	for _, section := range plan.Content() {
		for _, run := range section.Children()[0].Runs() {
			if run.Kind() == RunRef {
				refs = append(refs, strings.Join(run.RefPath(), "/")+"="+run.Text())
			}
		}
	}
	want = "power_budget=Power budget power_budget=Power budget MassBudget=Mass budget MassBudget=Mass budget"
	if got := strings.Join(refs, " "); got != want {
		t.Errorf("references %q, want %q", got, want)
	}
}

// A heading qualified by a usage typed by more than one document is an
// ambiguous target naming both documents, not an unknown one.
func TestCompileWholeDocumentAmbiguousDocumentUsage(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		ref both : First, Second;
		part def First :> Document {
			attribute redefines title = "First";
			rep language "markdown" /* # Overview */
		}
		part def Second :> Document {
			attribute redefines title = "Second";
			rep language "markdown" /* # Overview */
		}
		part def Report :> Document {
			attribute redefines title = "Report";
			rep language "markdown" /* See [](sysml:both::overview). */
		}
	`)
	_, err := fixture.compile(t, "Report")
	planning := bodyPlanningError(t, err)
	if planning.Kind != ErrorAmbiguousRefTarget {
		t.Fatalf("error = %v, want kind %s", err, ErrorAmbiguousRefTarget)
	}
	if !strings.Contains(planning.Error(), "Observatory::First and Observatory::Second") {
		t.Errorf("message = %q", planning.Error())
	}
	if planning.Line != 1 || !planning.Origin.Located() {
		t.Errorf("line %d, located %v", planning.Line, planning.Origin.Located())
	}
}

// An allocated name is never shadowed by a later heading's explicit id, a
// repeated id keeps naming its first heading, and a quoted name spells an id.
func TestCompileWholeDocumentSectionAliases(t *testing.T) {
	useRecordedAsciiDoc(t)
	fixture := loadPlanningFixture(t, `
	part def Report :> Document {
		attribute redefines title = "Report";
		rep language "markdown" /* # Overview */
		`+asciiDocBody(t, "asciidoc_aliases.adoc")+`
	}
`)
	plan := fixture.mustCompile(t, "Report")
	want := strings.TrimLeft(`
overview Overview
overview_2 Appendix
  paragraph
power_budget Power budget
power_budget_2 Reserve power
`, "\n")
	if got := outline(plan.Content(), 0); got != want {
		t.Fatalf("outline:\n%s\nwant:\n%s", got, want)
	}
	var refs []string
	for _, run := range plan.Content()[1].Children()[0].Runs() {
		if run.Kind() == RunRef {
			refs = append(refs, strings.Join(run.RefPath(), "/")+"="+run.Text())
		}
	}
	want = "overview=Overview power_budget=Power budget power_budget=Power budget"
	if got := strings.Join(refs, " "); got != want {
		t.Errorf("references %q, want %q", got, want)
	}
}

// A nested heading is also a target by its named path, in this document and
// qualified by a document definition from another.
func TestCompileWholeDocumentNestedSectionPaths(t *testing.T) {
	fixture := loadPlanningFixture(t, `
	part def Report :> Document {
		attribute redefines title = "Report";
		rep language "markdown" /* See [](sysml:overview.mass_budget) and [](sysml:appendix.derivation).
		 *
		 * # Overview
		 *
		 * ## Mass budget
		 */
		part appendix : Section {
			attribute redefines title = "Appendix";
			rep language "markdown" /* ## Derivation */
		}
	}
	part def Other :> Document {
		attribute redefines title = "Other";
		rep language "markdown" /* See [](sysml:Report::appendix.derivation) and [](sysml:Report::overview.mass_budget). */
	}
`)
	plan := fixture.mustCompile(t, "Report")
	want := "overview/mass_budget=Mass budget appendix/derivation=Derivation"
	if got := strings.Join(sectionRefs(plan.Content()), " "); got != want {
		t.Errorf("references %q, want %q", got, want)
	}
	plan = fixture.mustCompile(t, "Other")
	want = "Observatory::Report:appendix/derivation=Derivation Observatory::Report:overview/mass_budget=Mass budget"
	if got := strings.Join(sectionRefs(plan.Content()), " "); got != want {
		t.Errorf("cross-document references %q, want %q", got, want)
	}
	_, err := loadPlanningFixture(t, `
	part def Report :> Document {
		attribute redefines title = "Report";
		rep language "markdown" /* See [](sysml:overview.derivation).
		 *
		 * # Overview
		 */
		part appendix : Section {
			attribute redefines title = "Appendix";
			rep language "markdown" /* ## Derivation */
		}
	}
`).compile(t, "Report")
	if planning := bodyPlanningError(t, err); planning.Kind != ErrorUnknownRefTarget {
		t.Errorf("a path under the wrong section: %v, want kind %s", err, ErrorUnknownRefTarget)
	}
}

// sectionRefs lists every reference run under content as
// [document:]path=text, in document order.
func sectionRefs(content []Content) []string {
	var refs []string
	for _, node := range content {
		for _, run := range node.Runs() {
			if run.Kind() != RunRef {
				continue
			}
			ref := strings.Join(run.RefPath(), "/") + "=" + run.Text()
			if doc := run.RefDocument(); doc != "" {
				ref = doc + ":" + ref
			}
			refs = append(refs, ref)
		}
		refs = append(refs, sectionRefs(node.Children())...)
	}
	return refs
}
