package migrate_test

import (
	stderrors "errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docpdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

const icdDocument = "Observatory::Documents::'Interface Control Document Document'"

// TestDocGenExpressionColumnsLowered checks the OCL subset of a
// TableExpressionColumn is written as a Column computed over the row: a
// typed-by count, the parts a connector's nested ends pass through, the ends'
// types and direction; an expression outside the subset is refused quoted.
func TestDocGenExpressionColumnsLowered(t *testing.T) {
	r := migrateFixtureFile(t, "docgen_columns")
	for _, d := range errors(t, "docgen_columns.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
	wantLine(t, r.Notation, `DocumentQueries::Column(name = "Number of Interfaces", cell = { in row : KerML::Core::Type; DocumentQueries::RelatedElements(source = row, relationshipKind = "typing", direction = "incoming", maxDepth = 1)->SequenceFunctions::size() })`)
	wantInOrder(t, "connector columns", string(r.Notation),
		"calc def 'Interface Control Document Table of Connections Rows'",
		`DocumentQueries::Column(name = "Point of Interface on Optical Bench", cell = { in row : KerML::Kernel::Connector; row.connectorEnd->ControlFunctions::select {in e : KerML::Core::Feature; e.chainingFeature->SequenceFunctions::excluding(e.chainingFeature->SequenceFunctions::last())->ControlFunctions::exists {in y : KerML::Core::Feature; y.type.name == "Optical Bench"}}`,
		`->ControlFunctions::select {in z : KerML::Core::Feature; z.type.name != "Optical Bench"}.name }),`,
		`DocumentQueries::Column(name = "Point of Interface on Controller Rack", cell = { in row : KerML::Kernel::Connector;`,
		`DocumentQueries::Column(name = "Type", cell = { in row : KerML::Kernel::Connector; row.connectorEnd->ControlFunctions::collect {in e : KerML::Core::Feature; e.chainingFeature->SequenceFunctions::last()}.type->DocumentQueries::Distinct() }),`,
		`DocumentQueries::Column(name = "Direction", cell = { in row : KerML::Kernel::Connector; row.connectorEnd->ControlFunctions::collect {in e : KerML::Core::Feature; e.chainingFeature->SequenceFunctions::last()}.type.member->ControlFunctions::select {in y : KerML::Core::Feature; y.direction->SequenceFunctions::notEmpty()}.direction->DocumentQueries::Distinct() })`)
	wantOneNote(t, r, "_st_tot_table", migrate.Approximated,
		`the column «TableExpressionColumn» Observatory::Viewpoints::Totals Viewpoint::Totals Method::Interface Totals::Iterated is not written: the expression "ownedElement->iterate(x; acc : Integer = 0 | acc + 1)" is not lowered: the character ";" has no place in an OCL expression`)
	for _, id := range []string{"_st_tot_col_count", "_st_conn_col_bench", "_st_conn_col_rack", "_st_conn_col_type", "_st_conn_col_dir"} {
		for _, e := range entriesFor(r, id) {
			if strings.Contains(e.Note, "is not written") || e.Verdict == migrate.Unmapped {
				t.Errorf("column %s is ledgered refused: %+v", id, e)
			}
		}
	}
	if strings.Contains(string(r.Notation), `Column(name = "Iterated"`) {
		t.Errorf("the refused column is written:\n%s", r.Notation)
	}
}

// TestDocGenCollectedColumnsLowered checks a column whose chain collects
// before it reads is a Column collecting from the row and projecting the
// attribute of every element collected, while a chain with a step no query
// spells keeps refusing with the step quoted.
func TestDocGenCollectedColumnsLowered(t *testing.T) {
	r := migrateFixtureFile(t, "docgen_columns")
	wantLine(t, r.Notation, `DocumentQueries::Column(name = "Description", cell = { in row : KerML::Root::Element; DocumentQueries::WhereType(source = DocumentQueries::Descendants(source = row, maxDepth = 1), type = ("PortUsage")).documentation })`)
	wantLine(t, r.Notation, `DocumentQueries::Column(name = "Interfaces", cell = { in row : KerML::Root::Element; DocumentQueries::RelatedElements(source = DocumentQueries::WhereType(source = DocumentQueries::Descendants(source = row, maxDepth = 1), type = ("PortUsage")), relationshipKind = ("typing"), direction = ("outgoing"), maxDepth = 1).name })`)
	wantOneNote(t, r, "_st_ifc_sorted_sort", migrate.Unmapped, "no query operation or content block stands for «SortByProperty»")
	wantOneNote(t, r, "_st_ifc_table", migrate.Approximated,
		"the column «TableAttributeColumn» Observatory::Viewpoints::Interfaces Viewpoint::Interfaces Method::Per Assembly::Interfaces Table::Sorted Ports is not written: its chain is not lowered: «SortByProperty» Observatory::Viewpoints::Interfaces Viewpoint::Interfaces Method::Per Assembly::Interfaces Table::Sorted Ports::Sort By Property is not migrated: no query operation or content block stands for «SortByProperty»")
	for _, e := range entriesFor(r, "_st_doc_icd") {
		if n := strings.Count(e.Note, "Sorted Ports is not written"); n != 1 {
			t.Errorf("the document's note repeats the refused column %d time(s):\n%s", n, e.Note)
		}
	}
	if n := strings.Count(string(r.Notation), `Column(name = "Sorted Ports"`); n != 0 {
		t.Errorf("the refused column is written %d time(s):\n%s", n, r.Notation)
	}
}

// TestDocGenAssociationColumnRefused checks a column whose chain follows
// associations is refused with the step quoted: the step spells its result
// from the elements known at migration, which are every row's together, not
// each row's own.
func TestDocGenAssociationColumnRefused(t *testing.T) {
	r := migrateFixtureFile(t, "docgen_columns")
	const why = "it follows the composite attributes of each row to their types, and no query operation tells a composite feature from the others"
	wantOneNote(t, r, "_st_ifc_assoc_collect", migrate.Unmapped, why)
	wantOneNote(t, r, "_st_ifc_table", migrate.Approximated,
		"the column «TableAttributeColumn» Observatory::Viewpoints::Interfaces Viewpoint::Interfaces Method::Per Assembly::Interfaces Table::Associated is not written: its chain is not lowered: «CollectByAssociation» Observatory::Viewpoints::Interfaces Viewpoint::Interfaces Method::Per Assembly::Interfaces Table::Associated::Collect By Association is not migrated: "+why)
	if n := strings.Count(string(r.Notation), `Column(name = "Associated"`); n != 0 {
		t.Errorf("the refused column is written %d time(s):\n%s", n, r.Notation)
	}
}

// TestDocGenRefusalsLedgeredPerDocument checks a step two documents share is
// ledgered refused once for each document, and once however many passes of a
// loop in one document meet it.
func TestDocGenRefusalsLedgeredPerDocument(t *testing.T) {
	r := migrateFixtureFile(t, "docgen_columns")
	for _, id := range []string{"_st_ifc_sorted_sort", "_st_ifc_assoc_collect"} {
		var unmapped int
		for _, e := range entriesFor(r, id) {
			if e.Verdict == migrate.Unmapped {
				unmapped++
			}
		}
		if unmapped != 2 {
			t.Errorf("%s is ledgered unmapped %d time(s), want once per document:\n%+v", id, unmapped, entriesFor(r, id))
		}
	}
	for _, id := range []string{"_st_doc_icd", "_st_doc_sum"} {
		for _, e := range entriesFor(r, id) {
			if n := strings.Count(e.Note, "Sorted Ports is not written"); n != 1 {
				t.Errorf("%s notes the refused column %d time(s):\n%s", id, n, e.Note)
			}
		}
	}
	wantInOrder(t, "second document", string(r.Notation),
		"part def 'Interface Summary Document' :> DocumentQueries::Document {",
		"calc rows : 'Interface Summary Interfaces Table Rows';",
		"calc rows : 'Interface Summary Interfaces Table Rows 2';")
}

// TestDocGenLoopTableListsTogether checks a TableStructure with loop = true
// is one table over the elements its chain holds, ledgered mapped: DocGen
// keeps a table's loop and runs none, so one table is what it prints. The
// table nested in a looping StructuredQuery still lists each pass's elements.
func TestDocGenLoopTableListsTogether(t *testing.T) {
	r := migrateFixtureFile(t, "docgen_columns")
	wantInOrder(t, "looped table", string(r.Notation),
		"calc def 'Interface Control Document Per Type Rows'",
		`source = DocumentQueries::Named(qualifiedName = ("Observatory::Interface Types")),`,
		`DocumentQueries::Column(name = "Number of Interfaces"`,
		"part 'Interface Totals' : DocumentQueries::Section {",
		"calc rows : 'Interface Control Document Interface Totals Rows';",
		"part 'table 2' : DocumentQueries::Table {",
		`attribute redefines caption = "Per Type";`,
		"calc rows : 'Interface Control Document Per Type Rows';",
		"part Connections : DocumentQueries::Section {")
	if n := strings.Count(string(r.Notation), "calc def 'Interface Control Document Per Type Rows"); n != 1 {
		t.Errorf("the looped table is written as %d queries, want one over its elements together", n)
	}
	wantTarget(t, r, "_st_tot_each", migrate.Mapped, "part "+icdDocument+"::'Interface Totals'::'table 2'")
	for _, e := range entriesFor(r, "_st_tot_each") {
		if strings.Contains(e.Note, "loop") {
			t.Errorf("a table's loop, which DocGen runs none of, is noted: %s", e.Note)
		}
	}
	wantInOrder(t, "nested table", string(r.Notation),
		"calc def 'Interface Control Document Interfaces Table Rows'",
		`DocumentQueries::Named(qualifiedName = ("Observatory::Assemblies::Optical Bench::collimator"))`,
		"calc def 'Interface Control Document Interfaces Table Rows 2'",
		`DocumentQueries::Named(qualifiedName = ("Observatory::Assemblies::Optical Bench::mirror"))`)
}

// TestDocGenLoopsUnrolled checks a looping StructuredQuery writes its body
// once per element it holds, the element alone its target, and a looping
// Dynamic View one section per element, titled by the element.
func TestDocGenLoopsUnrolled(t *testing.T) {
	r := migrateFixtureFile(t, "docgen_columns")
	wantOneNote(t, r, "_st_ifc_none", migrate.Mapped, "it loops over no element: «FilterByMetaclasses» Observatory::Viewpoints::Interfaces Viewpoint::Interfaces Method::Filter By Metaclasses drops all 2 elements collected")
	wantOneNote(t, r, "_st_ifc_none", migrate.Mapped, "its body is not written, as DocGen writes nothing for it")
	if strings.Contains(string(r.Notation), "Interface Control Document Activity Rows") {
		t.Errorf("a loop over no element wrote its body's list query")
	}
	wantInOrder(t, "looped tables", string(r.Notation),
		"calc def 'Interface Control Document Assembly Rows'",
		`DocumentQueries::Named(qualifiedName = ("Observatory::Assemblies::Optical Bench::collimator"))`,
		"calc def 'Interface Control Document Interfaces Table Rows'",
		`DocumentQueries::Named(qualifiedName = ("Observatory::Assemblies::Optical Bench::collimator"))`,
		"calc def 'Interface Control Document Assembly Rows 2'",
		`DocumentQueries::Named(qualifiedName = ("Observatory::Assemblies::Optical Bench::mirror"))`,
		"calc def 'Interface Control Document Interfaces Table Rows 2'",
		`DocumentQueries::Named(qualifiedName = ("Observatory::Assemblies::Optical Bench::mirror"))`,
		"part Interfaces : DocumentQueries::Section {",
		"part list : DocumentQueries::List {",
		"calc items : 'Interface Control Document Assembly Rows';",
		"part table : DocumentQueries::Table {",
		`attribute redefines caption = "Interfaces Table";`,
		"calc rows : 'Interface Control Document Interfaces Table Rows';",
		"part 'list 2' : DocumentQueries::List {",
		"calc items : 'Interface Control Document Assembly Rows 2';",
		"part 'table 2' : DocumentQueries::Table {",
		`attribute redefines caption = "Interfaces Table";`,
		"calc rows : 'Interface Control Document Interfaces Table Rows 2';",
		"part 'Per Part' : DocumentQueries::Section {",
		"part collimator : DocumentQueries::Section {",
		`attribute redefines title = "collimator";`,
		"part mirror : DocumentQueries::Section {",
		`attribute redefines title = "mirror";`)
	if strings.Contains(string(r.Notation), "one table lists them together") {
		t.Errorf("a loop whose elements are known is collapsed:\n%s", r.Notation)
	}
	wantTarget(t, r, "_st_ifc_table", migrate.Approximated, "part "+icdDocument+"::Interfaces::table")
	wantTarget(t, r, "_st_ifc_table", migrate.Approximated, "part "+icdDocument+"::Interfaces::'table 2'")
	wantTarget(t, r, "_per_call", migrate.Mapped, "part "+icdDocument+"::'Per Part'::collimator")
	wantTarget(t, r, "_per_call", migrate.Mapped, "part "+icdDocument+"::'Per Part'::mirror")
}

// wantTarget asserts an entry for id holds verdict and is written as target.
func wantTarget(t *testing.T, r *migrate.Result, id string, verdict migrate.Verdict, target string) {
	t.Helper()
	es := entriesFor(r, id)
	for _, e := range es {
		if e.Verdict == verdict && e.Target == target {
			return
		}
	}
	t.Errorf("entries for %s = %+v, want a %v entry written as %q", id, es, verdict, target)
}

// TestDocGenColumnsRender renders the migrated document: the computed and
// collected cells hold what the model says when the queries run, a collection
// fills its cell with every value, and the looped table prints once per
// element behind the element's bullet, in Markdown and HTML alike.
func TestDocGenColumnsRender(t *testing.T) {
	r := migrateFixtureFile(t, "docgen_columns")
	s := session(t, r)
	md := markdown(t, s, icdDocument)
	wantInOrder(t, "Markdown", md,
		"# Interface Control Document",
		"## Interface Totals",
		"| name | Number of Interfaces |",
		"| Serial | 3 |",
		"| Spare | 0 |",
		"| USB | 2 |",
		"*Per Type*",
		"| name | Number of Interfaces |",
		"| Serial | 3 |",
		"| Spare | 0 |",
		"| USB | 2 |",
		"## Connections",
		"| documentation | Point of Interface on Optical Bench | Point of Interface on Controller Rack | Type | Direction |",
		"| Camera data to the rack. | collimator | hub | USB | inout |",
		"|  | mirror | hub | Serial | in |",
		"## Interfaces",
		"- collimator",
		"*Interfaces Table*",
		"| name | Description | Interfaces |",
		"| Collimator | Camera link. | USB, Serial |",
		"- mirror",
		"*Interfaces Table*",
		"| name | Description | Interfaces |",
		"| Mirror | Tilt command. | Serial |",
		"## Per Part",
		"### collimator",
		"- collimator",
		"### mirror",
		"- mirror")
	if strings.Count(md, "*Interfaces Table*") != 2 {
		t.Errorf("the looped table does not print once per assembly:\n%s", md)
	}
	if strings.Count(md, "*Per Type*") != 1 {
		t.Errorf("the table whose loop DocGen runs none of does not print once:\n%s", md)
	}
	h := html(t, s, icdDocument)
	wantInOrder(t, "HTML", h,
		"<h2>Interface Totals</h2>",
		">Number of Interfaces</th>",
		">Serial</span>", ">3</span>",
		`<caption class="sysml-caption">Per Type</caption>`, ">Serial</span>", ">3</span>", ">Spare</span>", ">0</span>", ">USB</span>", ">2</span>",
		"<h2>Connections</h2>",
		">Camera data to the rack.</span>", ">collimator</span>", ">hub</span>", ">USB</span>", ">inout</span>",
		"<h2>Interfaces</h2>",
		"<ul", ">collimator</li>",
		`<caption class="sysml-caption">Interfaces Table</caption>`,
		">Collimator</span>", ">Camera link.</span>",
		`>USB</span><span class="sysml-separator">, </span><span class="sysml-value" data-value-kind="string">Serial</span>`,
		"<ul", ">mirror</li>",
		`<caption class="sysml-caption">Interfaces Table</caption>`,
		">Mirror</span>", ">Tilt command.</span>", ">Serial</span>",
		"<h2>Per Part</h2>",
		"<h3>collimator</h3>", ">collimator</li>",
		"<h3>mirror</h3>", ">mirror</li>")
	if strings.Count(h, `<caption class="sysml-caption">Interfaces Table</caption>`) != 2 {
		t.Errorf("the looped table does not print once per assembly:\n%s", h)
	}
}

// TestDocGenLoopTablesRenderInstalledPDF renders the migrated document
// through the installed WeasyPrint, as the PDF toolchain job runs it: each
// pass of the looped table is a numbered table of its own behind the
// element's bullet. Without the toolchain it skips unless
// OPENSYSML_REQUIRE_PDF_TOOLCHAIN is set.
func TestDocGenLoopTablesRenderInstalledPDF(t *testing.T) {
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
	r := migrateFixtureFile(t, "docgen_columns")
	s := session(t, r)
	document, err := s.EvaluateDocument(icdDocument)
	if err != nil {
		t.Fatalf("evaluate %s: %v", icdDocument, err)
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
	text := strings.Join(strings.Fields(string(out)), " ")
	wantInOrder(t, "PDF text", text,
		"Interface Totals",
		"Number of Interfaces",
		"Serial", "3",
		"Table 2.", "Per Type", "Serial", "3", "Spare", "0", "USB", "2",
		"Connections",
		"Camera data to the", "collimator", "hub", "USB", "inout",
		"Interfaces",
		"collimator",
		"Table 4.", "Interfaces Table",
		"Collimator", "Camera link.", "USB, Serial",
		"mirror",
		"Table 5.", "Interfaces Table",
		"Mirror", "Tilt command.", "Serial")
}
