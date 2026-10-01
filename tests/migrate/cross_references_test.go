package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// The cross-references a tool stores in a comment's HTML — the View Editor's
// <mms-cf> spans and MagicDraw's mdel:// hyperlinks — are resolved through
// the export. In a document paragraph each is a Ref run: to the section of a
// view, the figure of a diagram, or the element itself; the prose between
// them is Span runs. A dangling <mms-cf> leaves nothing and a dangling
// hyperlink its text, both ledgered on the comment, and no `[cf:…]` fallback
// is printed anywhere.
func TestMigratedCrossReferences(t *testing.T) {
	r := migrateFixtureFile(t, "cross_references")
	wantClean(t, "cross_references.sysml", r)
	notation := string(r.Notation)
	wantInOrder(t, "notation", notationSection(notation, "Overview"),
		"part paragraph : DocumentQueries::Paragraph {",
		"part span : DocumentQueries::Span {",
		`attribute redefines text = "The";`,
		"part 'ref' : DocumentQueries::Ref {",
		"ref redefines target = Observatory::Mount.metadata;",
		`attribute redefines text = "is aligned in";`,
		"ref redefines target = Documents::'Observatory Handbook Document'::Overview.Alignment;",
		`attribute redefines text = "and drawn in";`,
		"ref redefines target = Documents::'Observatory Handbook Document'::Figures.diagram;",
		`attribute redefines text = "Mount Structure";`,
		`attribute redefines text = "(). Its focal length is 2.5; see Lost Procedure.\nOptics: Gathers the light. Coating: See";`,
		"ref redefines target = Documents::'Observatory Handbook Document'::Overview.Alignment;",
		`attribute redefines text = ".";`,
	)
	for _, stray := range []string{"[cf:", "Old Mount", "mms-cf", "mdel://"} {
		if strings.Contains(notation, stray) {
			t.Errorf("notation prints %q:\n%s", stray, notation)
		}
	}

	// Plain text — a doc comment, a requirement's Text — reads a reference as
	// the target's name and a dangling <mms-cf> as its cached text, unwrapped.
	wantLine(t, r.Notation, "doc /* Holds the Optics and the Table 7-4 steady. */")
	wantLine(t, r.Notation, "Optics: Gathers the light. Coating: See Alignment.")
	wantLine(t, r.Notation, "doc /* The Mount shall point within of the target. */")

	wantOneNote(t, r, "_overview_doc", migrate.Approximated, "the hyperlink 'Old Mount' reads 'Old Mount', not the current name 'Mount' of 'Mount', which is written")
	wantOneNote(t, r, "_overview_doc", migrate.Approximated, "a cross-reference to the name of an element names an element the export does not contain: MMS_1461107722575_gone; nothing stands for it")
	wantOneNote(t, r, "_overview_doc", migrate.Approximated, "the hyperlink 'Lost Procedure' names an element the export does not contain: _nowhere; its text stands")
	wantOneNote(t, r, "_overview_doc", migrate.Approximated, "a cross-reference to the value of 'focalLength' is written as the text of its value at migration time")
	wantOneNote(t, r, "_overview_doc", migrate.Approximated, "a cross-reference to the documentation of 'Optics' is written as its text at migration time")
	wantOneNote(t, r, "_overview_doc", migrate.Approximated, "a cross-reference to the value of 'coating' has no text: it holds no value")
	wantNote(t, r, "_cmt_mount", migrate.Approximated, "names an element the export does not contain: MMS_1461107722575_gone; its cached text 'Table 7-4' stands")
	wantNote(t, r, "_req_point", migrate.Approximated, "a cross-reference to the value of an element names an element the export does not contain: MMS_1461107722575_slot; nothing stands for it")
	for _, e := range entriesFor(r, "_overview_doc") {
		if n := strings.Count(e.Note, "'Lost Procedure'"); n != 1 {
			t.Errorf("the comment's entry notes the hyperlink %d times, want once:\n%s", n, e.Note)
		}
	}

	// Rendered, a reference to a section or figure of the document is a link
	// labelled as the renderer numbers it; one to an element is its name.
	s := session(t, r)
	const doc = "Documents::'Observatory Handbook Document'"
	md := markdown(t, s, doc)
	wantInOrder(t, "Markdown", md,
		"The Mount is aligned in [Alignment](#Overview-Alignment) and drawn in [Mount Structure](#Figures-diagram) (). Its focal length is 2.5; see Lost Procedure. Optics: Gathers the light. Coating: See [Alignment](#Overview-Alignment).",
		`<a id="Overview-Alignment"></a>`,
		`<a id="Figures-diagram"></a>`,
	)
	out, err := s.RenderDocumentHTML(doc, docrender.HTMLOptions{Fragment: true, NumberSections: true, NumberFigures: true})
	if err != nil {
		t.Fatalf("render %s as HTML: %v", doc, err)
	}
	wantInOrder(t, "HTML", out,
		`The <a class="sysml-ref" data-element="Observatory::Mount">Mount</a> is aligned in <a class="sysml-ref" href="#Overview-Alignment">Section 1.1 - Alignment</a> and drawn in <a class="sysml-ref" href="#Figures-diagram">Mount Structure</a> (). Its focal length is 2.5; see Lost Procedure.`,
		`id="Overview-Alignment"`,
		`id="Figures-diagram"`,
	)
}
