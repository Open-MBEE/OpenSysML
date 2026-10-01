package migrate_test

import (
	stderrors "errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docpdf"
	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// plantReport is the document the table_figures fixture migrates, whose views
// draw a Cameo table through an «Image», conform to no viewpoint, and carry
// collaborator paragraphs anchored to generated figures.
const plantReport = "'Plant Documents'::'Plant Report Document'"

// elementListing is the header of the generic member listing a view rendered
// asElementTable draws; a document embedding a Cameo table never shows it.
const elementListing = "Declared in"

func plantReportResult(t *testing.T) *migrate.Result {
	t.Helper()
	r := migrateFixtureFile(t, "table_figures")
	for _, d := range errors(t, "table_figures.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
	return r
}

func TestImageOfTableEmbedsTheTable(t *testing.T) {
	r := plantReportResult(t)
	notation := string(r.Notation)
	sec := notationSection(notation, "Inventory")
	if sec == "" {
		t.Fatalf("no Inventory section written:\n%s", notation)
	}
	wantInOrder(t, "Inventory section", sec,
		"part table : DocumentQueries::Table {",
		`attribute redefines caption = "Pump Inventory";`,
		"calc rows : Plant::Inventory::'Pump Table Rows';")
	if strings.Contains(sec, "DocumentQueries::Diagram") {
		t.Errorf("the Image of the table draws the view as a Diagram:\n%s", sec)
	}
	if n := strings.Count(notation, "calc def 'Pump Table Rows'"); n != 1 {
		t.Errorf("the table's query is written %d times, want once:\n%s", n, notation)
	}
	wantInOrder(t, "standalone table document", notation,
		"part def 'Pump Table Document' :> DocumentQueries::Document {",
		"part rows : DocumentQueries::Table {",
		`attribute redefines caption = "Pump Table";`,
		"calc rows : 'Pump Table Rows';")
	wantOneNote(t, r, "_st_titled_image", migrate.Mapped, "the SysML Instance Table 'Pump Table' is written as a Table over the query 'Pump Table Rows' of its «InstanceTable»")
	wantOneNote(t, r, "_st_titled_image", migrate.Mapped, "the paragraph is the Table's caption")
}

// A table whose diagram a view owns has its query inside that view usage; a
// section elsewhere reaches it by qualified name, as a definition is reached.
func TestImageOfTableOwnedByViewEmbedsTheTable(t *testing.T) {
	r := plantReportResult(t)
	notation := string(r.Notation)
	wantInOrder(t, "Gallery view", notation,
		"view Gallery {",
		"calc def 'Block Table Rows' :> DocumentQueries::Query {",
		"part def 'Block Table Document' :> DocumentQueries::Document {",
		"calc rows : 'Block Table Rows';")
	sec := notationSection(notation, "Gallery")
	if sec == "" {
		t.Fatalf("no Gallery section written:\n%s", notation)
	}
	wantInOrder(t, "Gallery section", sec,
		`attribute redefines text = "Every block of the plant.";`,
		"part table : DocumentQueries::Table {",
		`attribute redefines caption = "Block Table";`,
		"calc rows : 'Plant Documents'::Gallery::'Block Table Rows';")
	if strings.Contains(sec, "DocumentQueries::Diagram") {
		t.Errorf("the Image of the table draws the view as a Diagram:\n%s", sec)
	}
	wantOneNote(t, r, "_st_plain_image", migrate.Mapped, "the Generic Table 'Block Table' is written as a Table over the query 'Block Table Rows' of its «DiagramTable»")
}

func TestImageOfRefusedTableIsRefused(t *testing.T) {
	r := plantReportResult(t)
	sec := notationSection(string(r.Notation), "Spares")
	if sec == "" {
		t.Fatalf("no Spares section written:\n%s", r.Notation)
	}
	wantInOrder(t, "Spares section", sec,
		`attribute redefines text = "Spare parts of the plant.";`,
		"/* not migrated: «Image» CallBehaviorAction 'Image' — the «DiagramTable» 'Spare Parts' has no query form, so the table is left out rather than shown as a listing of its view's elements: the table names no scope and no rows */",
		`attribute redefines text = "Where the table would be.";`)
	for _, kind := range []string{"DocumentQueries::Diagram", "DocumentQueries::Table"} {
		if strings.Contains(sec, kind) {
			t.Errorf("the refused table is drawn as a %s:\n%s", kind, sec)
		}
	}
	wantOneNote(t, r, "_st_plain_image", migrate.Unmapped, "has no query form, so the table is left out rather than shown as a listing of its view's elements")
}

func TestViewpointLessViewShowsExposedDiagrams(t *testing.T) {
	r := plantReportResult(t)
	notation := string(r.Notation)
	sec := notationSection(notation, "Overview")
	if sec == "" {
		t.Fatalf("no Overview section written:\n%s", notation)
	}
	wantInOrder(t, "Overview section", sec,
		`attribute redefines text = "The plant at a glance.";`,
		"part table : DocumentQueries::Table {",
		`attribute redefines caption = "Pump Table";`,
		"calc rows : Plant::Inventory::'Pump Table Rows';",
		"part diagram : DocumentQueries::Diagram {",
		`attribute redefines caption = "Pump Structure";`,
		"ref redefines source = Plant::Structure::'Pump Structure';")
	if strings.Contains(sec, "Moves fluid through the plant.") {
		t.Errorf("the exposed block's documentation is shown, which DocGen does not do:\n%s", sec)
	}
	if n := strings.Count(sec, "DocumentQueries::Paragraph"); n != 1 {
		t.Errorf("Overview writes %d paragraphs, want its documentation alone:\n%s", n, sec)
	}
	details := notationSection(notation, "Details")
	if details == "" || strings.Contains(details, "DocumentQueries::Paragraph") || strings.Contains(details, "DocumentQueries::Diagram") || strings.Contains(details, "DocumentQueries::Table") || strings.Contains(details, "not migrated") {
		t.Errorf("the child view conforming to a viewpoint that draws nothing is not an empty heading:\n%s", details)
	}
	wantOneNote(t, r, "_view_overview", migrate.Mapped, "the view Plant Documents::Overview conforms to no viewpoint, so DocGen's default behavior applies: after the view's documentation it shows the SysML Instance Table 'Pump Table', the SysML Block Definition Diagram 'Pump Structure'; it shows nothing for the «Block» Class Plant::Structure::Pump, which is exposed but is not a diagram")
	wantOneNote(t, r, "_st_view_overview", migrate.Mapped, "the SysML Instance Table 'Pump Table' is written as a Table over the query 'Pump Table Rows' of its «InstanceTable»")
}

func TestViewWithBrokenConformIsRefused(t *testing.T) {
	r := plantReportResult(t)
	notation := string(r.Notation)
	sec := notationSection(notation, "Unlinked")
	if sec == "" {
		t.Fatalf("no Unlinked section written:\n%s", notation)
	}
	wantInOrder(t, "Unlinked section", sec,
		`/* not migrated: the view Plant Documents::Unlinked's conformance is not migrated: Conform general "_vp_missing" names no element */`,
		`attribute redefines text = "Its viewpoint is gone.";`)
	for _, kind := range []string{"DocumentQueries::Diagram", "DocumentQueries::Table"} {
		if strings.Contains(sec, kind) {
			t.Errorf("the view whose Conform names no viewpoint draws a %s as if it had none:\n%s", kind, sec)
		}
	}
	wantOneNote(t, r, "_st_view_unlinked", migrate.Unmapped, `the view Plant Documents::Unlinked's conformance is not migrated: Conform general "_vp_missing" names no element`)
	for _, e := range entriesFor(r, "_view_unlinked") {
		if strings.Contains(e.Note, "default behavior") {
			t.Errorf("_view_unlinked: %+v, want no default-behavior note", e)
		}
	}
}

func TestCollaboratorParagraphsFollowTheirAnchors(t *testing.T) {
	r := plantReportResult(t)
	notation := string(r.Notation)
	wantInOrder(t, "Pumping section", notationSection(notation, "Pumping"),
		`attribute redefines text = "How pumping works.";`,
		`attribute redefines text = "The following figure shows the pump.";`,
		"part diagram : DocumentQueries::Diagram {",
		`attribute redefines text = "The pump moves fluid.";`,
		`attribute redefines text = "It never runs dry.";`,
		`attribute redefines text = "The pump table is not drawn here.";`,
		`attribute redefines text = "Table anchors are not placed.";`)
	wantOneNote(t, r, "_st_pump_undrawn", migrate.Approximated, "its anchor names the figure of the SysML Instance Table 'Pump Table', which the section's method does not draw, so the paragraph follows the section's generated content")
	wantOneNote(t, r, "_st_pump_table", migrate.Approximated, `its anchor "Containment_TableMainImage___diag_pumps" names an item of the kind TableMainImage, which the migration does not place, so the paragraph follows the section's generated content`)
	for _, id := range []string{"_st_pump_head", "_st_pump_after", "_st_pump_follow"} {
		for _, e := range entriesFor(r, id) {
			if e.Verdict != migrate.Mapped || e.Note != "" {
				t.Errorf("%s: %+v, want Mapped without a note", id, e)
			}
		}
	}

	wantInOrder(t, "Inventory section", notationSection(notation, "Inventory"),
		`attribute redefines text = "Pumps on hand.";`,
		`attribute redefines text = "Every pump on hand.";`,
		"part table : DocumentQueries::Table {",
		`attribute redefines text = "Every pump the plant holds.";`,
		`attribute redefines text = "Read across each row.";`)
	wantOneNote(t, r, "_st_inv_note", migrate.Approximated, `its anchor "Containment_DiagramMainImage__7c1e4b" names no diagram of the model; the paragraph is placed after the section's only figure, of the SysML Instance Table 'Pump Table'`)

	wantInOrder(t, "Spares section", notationSection(notation, "Spares"),
		"/* not migrated: «Image» CallBehaviorAction 'Image'",
		`attribute redefines text = "Where the table would be.";`)
	for _, e := range entriesFor(r, "_st_spares_note") {
		if e.Verdict != migrate.Mapped || e.Note != "" {
			t.Errorf("_st_spares_note: %+v, want Mapped without a note", e)
		}
	}
}

func TestEmbeddedTablesRenderHTML(t *testing.T) {
	r := plantReportResult(t)
	s := session(t, r)
	page, err := s.RenderDocumentHTML(plantReport, docrender.HTMLOptions{Fragment: true, NumberSections: true, NumberFigures: true})
	if err != nil {
		t.Fatalf("render as HTML: %v", err)
	}
	if strings.Contains(page, elementListing) {
		t.Errorf("the HTML lists the table view's elements:\n%s", page)
	}
	wantInOrder(t, "HTML", page,
		"Inventory",
		"Pumps on hand.", "Every pump on hand.",
		`<table class="sysml-table" data-content="table" data-name="table" data-query="Plant::Inventory::Pump Table Rows">`,
		"Table 1.", "Pump Inventory",
		`<th scope="col" data-column="name">name</th>`, `data-column="mass">mass</th>`, `data-column="flow">flow</th>`,
		">p1</span>", ">12.5</span>", ">3</span>",
		">p2</span>", ">9</span>",
		"Every pump the plant holds.", "Read across each row.",
		"Spares", "Spare parts of the plant.", "Where the table would be.",
		"Overview", "The plant at a glance.",
		"Table 2.", "Pump Table",
		"Figure 1.", "Pump Structure",
		"Details",
		"Pumping", "How pumping works.", "The following figure shows the pump.",
		"Figure 2.", "Pump Structure",
		"The pump moves fluid.", "It never runs dry.",
		"Gallery", "Every block of the plant.",
		`<table class="sysml-table" data-content="table" data-name="table" data-query="Plant Documents::Gallery::Block Table Rows">`,
		"Table 3.", "Block Table",
		`<th scope="col" data-column="name">name</th>`,
		">Pump</span>", ">Valve</span>")
	if strings.Contains(page, "Table 4.") || strings.Contains(page, "Figure 3.") {
		t.Errorf("tables and figures are not numbered apart:\n%s", page)
	}
}

// TestEmbeddedTablesRenderInstalledPDF renders the fixture through the
// installed WeasyPrint, as the PDF toolchain job runs it; without the
// toolchain it skips unless OPENSYSML_REQUIRE_PDF_TOOLCHAIN is set.
func TestEmbeddedTablesRenderInstalledPDF(t *testing.T) {
	const engine = "weasyprint"
	converter, err := docpdf.EngineNamed(engine)
	if err != nil {
		t.Fatal(err)
	}
	if err := converter.Available(); err != nil {
		var docErr *docpdf.Error
		if !stderrors.As(err, &docErr) || docErr.Kind != docpdf.ErrorToolMissing {
			t.Fatal(err)
		}
		skipWithoutTool(t, engine, err)
	}
	pdftotext, err := exec.LookPath("pdftotext")
	if err != nil {
		skipWithoutTool(t, "pdftotext", err)
	}
	r := plantReportResult(t)
	s := session(t, r)
	document, err := s.EvaluateDocument(plantReport)
	if err != nil {
		t.Fatalf("evaluate %s: %v", plantReport, err)
	}
	pdf, err := docpdf.Render(document, engine, docpdf.Options{NumberSections: true, NumberFigures: true})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF-") {
		t.Fatalf("output is no PDF: %.16q", pdf)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "doc.pdf"), pdf, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(pdftotext, "-layout", filepath.Join(dir, "doc.pdf"), "-").Output() // #nosec G204 -- pdftotext from PATH, fixed arguments
	if err != nil {
		t.Fatalf("pdftotext: %v", err)
	}
	text := string(out)
	if strings.Contains(text, elementListing) {
		t.Errorf("the PDF lists the table view's elements:\n%s", text)
	}
	wantInOrder(t, "PDF text", text,
		"Every pump on hand.",
		"Table 1.", "Pump Inventory",
		"name", "mass", "flow",
		"p1", "12.5", "3",
		"p2", "9",
		"Every pump the plant holds.", "Read across each row.",
		"Where the table would be.",
		"The plant at a glance.",
		"Table 2.", "Pump Table",
		"Figure 1.", "Pump Structure",
		"The following figure shows the pump.",
		"Figure 2.", "Pump Structure",
		"The pump moves fluid.", "It never runs dry.",
		"Every block of the plant.",
		"Table 3.", "Block Table",
		"name", "Pump", "Valve")
}

// skipWithoutTool skips the test for a converter that is not installed, or
// fails it when the toolchain is declared mandatory.
func skipWithoutTool(t *testing.T, what string, err error) {
	t.Helper()
	const required = "OPENSYSML_REQUIRE_PDF_TOOLCHAIN"
	if v := os.Getenv(required); v != "" {
		t.Fatalf("%s=%s but %s not installed: %v", required, v, what, err)
	}
	t.Skipf("%s not installed: %v", what, err)
}
