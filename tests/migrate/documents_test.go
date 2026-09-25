package migrate_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// rows runs a migrated document query in the session and returns its report,
// failing unless the query executed.
func rows(t *testing.T, s *repl.Session, name string) string {
	t.Helper()
	v := s.RunDocumentQuery(name)
	if !v.Holds() {
		t.Fatalf("%s did not run:\n%s", name, strings.Join(v.Lines, "\n"))
	}
	return strings.Join(v.Lines, "\n")
}

// wantInOrder asserts the wanted strings appear in got in the order given.
func wantInOrder(t *testing.T, what, got string, want ...string) {
	t.Helper()
	rest := got
	for _, w := range want {
		i := strings.Index(rest, w)
		if i < 0 {
			t.Fatalf("%s lacks %q (or has it out of order):\n%s", what, w, got)
		}
		rest = rest[i+len(w):]
	}
}

// wantOneNote asserts that one of the entries reported for id has the verdict
// and notes the text, where the node is reported once per thing it produces.
func wantOneNote(t *testing.T, r *migrate.Result, id string, verdict migrate.Verdict, note string) {
	t.Helper()
	es := entriesFor(r, id)
	for _, e := range es {
		if e.Verdict == verdict && strings.Contains(e.Note, note) {
			return
		}
	}
	t.Errorf("entries for %s = %+v, want a %v entry noting %q", id, es, verdict, note)
}

// notationSection is the written Section of the title (quoted when it needs
// to be) up to the next Section, or "" when the notation writes none.
func notationSection(notation, title string) string {
	head := "part " + title + " : DocumentQueries::Section {"
	i := strings.Index(notation, head)
	if i < 0 {
		head = "part '" + title + "' : DocumentQueries::Section {"
		i = strings.Index(notation, head)
	}
	if i < 0 {
		return ""
	}
	body := notation[i:]
	if j := strings.Index(body[len(head):], ": DocumentQueries::Section {"); j >= 0 {
		body = body[:len(head)+j]
	}
	return body
}

// markdownSection is the Markdown from heading to the next heading of its
// level, or "" when the document has no such heading.
func markdownSection(md, heading string) string {
	i := strings.Index(md, heading)
	if i < 0 {
		return ""
	}
	body := md[i:]
	level := heading[:strings.IndexByte(heading, ' ')+1]
	if j := strings.Index(body[len(heading):], "\n"+level); j >= 0 {
		body = body[:len(heading)+j]
	}
	return body
}

// markdown renders a migrated document through the Markdown backend.
func markdown(t *testing.T, s *repl.Session, name string) string {
	t.Helper()
	out, err := s.RenderDocumentMarkdown(name, docrender.MarkdownOptions{})
	if err != nil {
		t.Fatalf("render %s as Markdown: %v", name, err)
	}
	return out
}

// html renders a migrated document through the HTML backend as a fragment.
func html(t *testing.T, s *repl.Session, name string) string {
	t.Helper()
	out, err := s.RenderDocumentHTML(name, docrender.HTMLOptions{Fragment: true})
	if err != nil {
		t.Fatalf("render %s as HTML: %v", name, err)
	}
	return out
}

// The queries a Cameo instance table, generic table, dependency matrix and
// relation map lower to execute over the migrated model and return the rows
// the tool showed: the instance rows of the scope plus the explicit rows,
// sorted as the table was, with the matrix cells naming the related elements.
func TestMigratedTablesExecute(t *testing.T) {
	s := session(t, migrateFixtureFile(t, "tables"))

	// Instance table: individuals of Pump (and its subtypes) under the scope
	// plus two explicit rows, sorted by mass descending with the empty cell last.
	pumps := rows(t, s, "Plant::Inventory::'Pump Table Rows'")
	wantInOrder(t, "Pump Table rows", pumps,
		"returned 5 rows",
		"Plant::Inventory::r1", `mass = 14`,
		"Plant::Inventory::p1", `mass = 12.5`, `flow = 3`,
		"Plant::Inventory::p2", `mass = 9`,
		"Plant::Spares::s1", `mass = 7`,
		"Plant::Spares::s2", `mass = ""`)
	if strings.Contains(pumps, "Plant::Inventory::v1") {
		t.Fatalf("Pump Table lists the valve v1:\n%s", pumps)
	}

	// Built-in columns are projected before the feature columns, and a
	// feature column captioned like a built-in property takes the suffixed name.
	ledger := rows(t, s, "Plant::Inventory::'Pump Ledger Rows'")
	wantInOrder(t, "Pump Ledger rows", ledger,
		"Columns: name, qualifiedName, name 2, mass",
		"Plant::Inventory::p1", `name = "p1"`, `qualifiedName = "Plant::Inventory::p1"`, `name 2 = "primary"`, `mass = 12.5`,
		"Plant::Inventory::p2", `name = "p2"`, `qualifiedName = "Plant::Inventory::p2"`, `name 2 = ""`)

	// Generic table: every requirement definition of the scope by name.
	reqs := rows(t, s, "Plant::Requirements::'Requirement Table Rows'")
	wantInOrder(t, "Requirement Table rows", reqs,
		"returned 3 rows",
		"FlowRequirement", "The pump keeps the flow above the minimum.",
		"MassRequirement", "SealRequirement")

	// Dependency matrix: the cell of each requirement row lists the blocks
	// that satisfy it, and the duplicate criterion column stays distinct.
	matrix := rows(t, s, "Plant::Requirements::'Satisfaction Matrix Rows'")
	wantInOrder(t, "Satisfaction Matrix rows", matrix,
		"Columns: name, Trace, Trace 2",
		"FlowRequirement", "Trace = Plant::Structure::Pump", "Trace 2 = (none)",
		"SealRequirement", "Trace = Plant::Structure::Valve",
		"MassRequirement", "Trace = (none)")

	// Relation map: the requirements one satisfaction hop from the context.
	related := rows(t, s, "Plant::Structure::'Pump Requirement Map Rows'")
	wantInOrder(t, "Pump Requirement Map rows", related,
		"returned 1 row",
		"Plant::Requirements::FlowRequirement", `@type = "RequirementDefinition"`)

	// Whole-model scope filtered by a migrated user stereotype.
	critical := rows(t, s, "Plant::'Critical Elements Rows'")
	wantInOrder(t, "Critical Elements rows", critical,
		"returned 2 rows", "Plant::Structure::Pump", "Plant::Requirements::FlowRequirement")
}

// A generic table over a broad UML metaclass lists what that metaclass holds
// in the source model: the packageable elements of a package but not the
// features they own, and the «View» and «Viewpoint» classes among the types.
func TestMetaclassTablesExecute(t *testing.T) {
	s := session(t, migrateFixtureFile(t, "metaclass_tables"))

	packageable := rows(t, s, "Tables::'Packageable Elements Rows'")
	wantInOrder(t, "Packageable Elements rows", packageable,
		"returned 11 rows",
		"Plant::Structure\n", "Plant::Structure::Mode\n", "Plant::Structure::Pump\n",
		"Plant::Structure::Pump::Cycle\n", "Plant::Structure::Pump::prime\n", "Plant::Structure::Valve\n",
		"Plant::Structure::needs\n", "Plant::Structure::p1\n",
		"Plant::Views\n", "Plant::Views::Operations\n", "Plant::Views::Overview\n")
	for _, feature := range []string{"Pump::mass", "Pump::valve", "Pump::'prime 2'", "Mode::on", "Cycle::Idle", "p1::mass"} {
		if strings.Contains(packageable, feature) {
			t.Errorf("Packageable Elements lists the owned feature %s:\n%s", feature, packageable)
		}
	}

	namespaces := rows(t, s, "Tables::'Namespaces Rows'")
	wantInOrder(t, "Namespaces rows", namespaces,
		"returned 12 rows",
		"Plant::Structure\n", "Plant::Structure::Mode\n", "Plant::Structure::Pump\n",
		"Plant::Structure::Pump::Cycle\n", "Plant::Structure::Pump::Cycle::Idle\n",
		"Plant::Structure::Pump::Cycle::Running\n", "Plant::Structure::Pump::prime\n",
		"Plant::Structure::Valve\n", "Plant::Structure::p1\n",
		"Plant::Views\n", "Plant::Views::Operations\n", "Plant::Views::Overview\n")
	if strings.Contains(namespaces, "Plant::Structure::needs") {
		t.Errorf("Namespaces lists the dependency needs:\n%s", namespaces)
	}

	for _, name := range []string{"Types", "Classifiers"} {
		got := rows(t, s, "Tables::'"+name+" Rows'")
		wantInOrder(t, name+" rows", got,
			"returned 8 rows",
			"Plant::Structure::Mode\n", "Plant::Structure::Pump\n", "Plant::Structure::Pump::Cycle\n",
			"Plant::Structure::Pump::prime\n", "Plant::Structure::Valve\n", "Plant::Structure::p1\n",
			"Plant::Views::Operations\n", "Plant::Views::Overview\n")
		for _, other := range []string{"Plant::Structure\n", "Plant::Views\n", "Cycle::Idle", "needs"} {
			if strings.Contains(got, other) {
				t.Errorf("%s lists %q, which is no type:\n%s", name, strings.TrimSpace(other), got)
			}
		}
	}

	// A whole-model table over Diagram lists every view, the one the model
	// itself owns — written at the top level — included.
	diagrams := rows(t, s, "Tables::'Diagrams Rows'")
	wantInOrder(t, "Diagrams rows", diagrams,
		"returned 7 rows",
		"Row 1: 'Model Overview'\n", "Plant::Views::Overview\n", "Tables::Classifiers\n", "Tables::Diagrams\n",
		"Tables::Namespaces\n", "Tables::'Packageable Elements'\n", "Tables::Types\n")
}

// A criterion that excludes subtypes of its stereotype cannot be told apart
// from one that includes them once every «Satisfy» — a user «Fulfil»
// specializing it included — is written as the same satisfy: the query lists
// the «Fulfil» relationships too, and the report says so. A criterion no
// applied stereotype specializes is exact either way.
func TestRelationCriterionSubtypes(t *testing.T) {
	r := migrateFixtureFile(t, "relation_subtypes")
	walked := "excludes subtypes of «Satisfy», but the «Fulfil» relationships are walked too"
	wantNote(t, r, "_mx_exact", migrate.Approximated, "the criterion Satisfied by "+walked)
	wantNote(t, r, "_map_valve", migrate.Approximated, "the criterion Satisfy "+walked)
	wantNote(t, r, "_mx_wide", migrate.Mapped, "")
	if es := entriesFor(r, "_mx_wide"); len(es) == 1 && strings.Contains(es[0].Note, "subtypes") {
		t.Errorf("Wide Satisfaction Matrix notes subtypes: %s", es[0].Note)
	}

	s := session(t, r)
	exact := rows(t, s, "Plant::Requirements::'Exact Satisfaction Matrix Rows'")
	wantInOrder(t, "Exact Satisfaction Matrix rows", exact,
		"FlowRequirement", "Satisfied by = Plant::Structure::Pump",
		"SealRequirement", "Satisfied by = Plant::Structure::Valve")
	wide := rows(t, s, "Plant::Requirements::'Wide Satisfaction Matrix Rows'")
	wantInOrder(t, "Wide Satisfaction Matrix rows", wide,
		"FlowRequirement", "Satisfied by = Plant::Structure::Pump", "Derived by = Plant::Requirements::SealRequirement",
		"SealRequirement", "Satisfied by = Plant::Structure::Valve", "Derived by = (none)")
	reached := rows(t, s, "Plant::Structure::'Valve Requirement Map Rows'")
	wantInOrder(t, "Valve Requirement Map rows", reached,
		"returned 1 row", "Plant::Requirements::SealRequirement")

	// A v1 DeriveReqt runs from the derived requirement to its original, a v2
	// derivation the other way: following it from the derived one reaches the original.
	original := rows(t, s, "Plant::Requirements::'Seal Derivation Map Rows'")
	wantInOrder(t, "Seal Derivation Map rows", original,
		"returned 1 row", "Plant::Requirements::FlowRequirement")
}

// A table whose diagram the model itself owns is written at the top level,
// beside its view, as a table in a package is beside its own.
func TestTopLevelTableIsWritten(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/tables.xmi")
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.ReplaceAll(data, []byte(`ownerOfDiagram="_pkg_inventory"`), []byte(`ownerOfDiagram="_m"`))
	r, err := migrate.Migrate("tables.xmi", data)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	wantClean(t, "tables.sysml", r)
	wantInOrder(t, "top-level table", string(r.Notation),
		"\nview 'Pump Table' {", "expose 'Pump Table Document';",
		"\ncalc def 'Pump Table Rows' :> DocumentQueries::Query {",
		"\npart def 'Pump Table Document' :> DocumentQueries::Document {", "calc rows : 'Pump Table Rows';")
	wantNote(t, r, "_tbl_pumps", migrate.Approximated, "written as a Document holding a Table over the query 'Pump Table Rows'")
	if es := entriesFor(r, "_tbl_pumps"); len(es) == 1 && es[0].Target != "part def 'Pump Table Document'" {
		t.Errorf("target = %q", es[0].Target)
	}
	wantInOrder(t, "top-level Pump Table rows", rows(t, session(t, r), "'Pump Table Rows'"),
		"returned 5 rows", "Plant::Inventory::r1", "Plant::Spares::s2")
}

// The Document each table becomes renders through the real Markdown and HTML
// backends with the executed rows in it.
func TestMigratedTablesRender(t *testing.T) {
	s := session(t, migrateFixtureFile(t, "tables"))

	md := markdown(t, s, "Plant::Inventory::'Pump Table Document'")
	wantInOrder(t, "Pump Table Markdown", md,
		"# Pump Table",
		"| name | mass | flow |",
		"| r1 | 14 |  |",
		"| p1 | 12.5 | 3 |",
		"| p2 | 9 |  |",
		"| s1 | 7 |  |",
		"| s2 |  |  |")

	page := html(t, s, "Plant::Inventory::'Pump Table Document'")
	wantInOrder(t, "Pump Table HTML", page,
		`<h1 class="sysml-title">Pump Table</h1>`,
		`data-query="Plant::Inventory::Pump Table Rows"`,
		`<th scope="col" data-column="mass">mass</th>`,
		`data-element="Plant::Inventory::r1"`, `>14</span>`,
		`data-element="Plant::Inventory::p1"`, `>12.5</span>`,
		`data-element="Plant::Spares::s2"`)

	matrix := markdown(t, s, "Plant::Requirements::'Satisfaction Matrix Document'")
	wantInOrder(t, "Satisfaction Matrix Markdown", matrix,
		"| name | Trace | Trace 2 |",
		"| FlowRequirement | Plant::Structure::Pump |  |",
		"| SealRequirement | Plant::Structure::Valve |  |",
		"| MassRequirement |  |  |")
}

// A DocGen document renders as the Section tree its views formed, each
// presentation node executing its query: lists, query-backed paragraphs,
// tables with property columns and view-backed diagrams all carry content,
// and a refused node or method leaves its enclosing section in place.
func TestMigratedDocumentsRender(t *testing.T) {
	r := migrateFixtureFile(t, "documents")
	wantNote(t, r, "_st_vp_headless", migrate.Unmapped,
		`the viewpoint Fleet Viewpoints::Headless Viewpoint's method is not migrated: method "_act_vanished" names no element`)
	wantInOrder(t, "headless section", string(r.Notation),
		"part Headless : DocumentQueries::Section {",
		`/* not migrated: the viewpoint Fleet Viewpoints::Headless Viewpoint's method is not migrated: method "_act_vanished" names no element */`)
	// Several name patterns are one WhereName, so the rows keep their order.
	wantInOrder(t, "name filter", string(r.Notation),
		"calc def 'Fleet Handbook Requirement List Rows'",
		`value = "^(?:Axle.*)$|^(?:Brake.*)$|^(?:Load.*)$"),`,
		"calc def 'Fleet Handbook Requirement Texts Rows'")
	// A table's or figure's caption is the title DocGen prints over it; the
	// captions text follows as a paragraph while showCaptions holds.
	wantInOrder(t, "captions", string(r.Notation),
		`attribute redefines caption = "Fleet Parts";`,
		`attribute redefines text = "The parts of the fleet, by name.";`,
		`attribute redefines caption = "Safety Requirements";`,
		"calc rows : 'Fleet Handbook Safety Requirements Rows';\n                }\n            }",
		`attribute redefines caption = "Truck Structure";`,
		`attribute redefines text = "The truck and what it hauls";`,
		`attribute redefines caption = "Figure: Inside the truck";`,
		"ref redefines source = truck.'Truck Internals';\n            }\n        }")
	if strings.Contains(string(r.Notation), "showCaptions is false") {
		t.Fatalf("a caption DocGen hides is written:\n%s", r.Notation)
	}
	s := session(t, r)

	md := markdown(t, s, "'Fleet Documents'::'Fleet Handbook Document'")
	wantInOrder(t, "Fleet Handbook Markdown", md,
		"# Fleet Handbook",
		"## Introduction",
		"*Fleet Parts*",
		"| name | qualifiedName | documentation | Payload | name 2 |",
		"| Axle | Fleet::Structure::Axle |  |  |  |",
		"| Trailer | Fleet::Structure::Trailer | Carries the load. |  |  |",
		"| Truck | Fleet::Structure::Truck | Hauls one trailer. |  |  |",
		"The parts of the fleet, by name.",
		"## Requirements",
		"Every truck of the fleet satisfies these requirements.",
		"1. Load Limit\n2. Brake Distance\n3. Axle Count",
		"The payload stays under the axle rating. A loaded truck stops within the legal distance. A truck has two axles.",
		"### Safety",
		"| Brake Distance | A loaded truck stops within the legal distance. |",
		"## Figures",
		"*Truck Structure*",
		"```mermaid",
		"The truck and what it hauls",
		"## Traceability",
		"- Truck",
		"## Oddities",
		"### Per Truck",
		"One truck.",
		"## Broken",
		"## Severed",
		"## Headless",
		"## Notes")
	if strings.Contains(md, "Axle Count | ") {
		t.Fatalf("the Safety table lists a requirement outside the Safety filter:\n%s", md)
	}
	if strings.Contains(md, "Cut off here.") {
		t.Fatalf("a method with a dangling control flow is written up to the break instead of refused:\n%s", md)
	}

	page := html(t, s, "'Fleet Documents'::'Fleet Handbook Document'")
	wantInOrder(t, "Fleet Handbook HTML", page,
		`<h1 class="sysml-title">Fleet Handbook</h1>`,
		"Introduction",
		`data-element="Fleet::Structure::Truck"`, "Hauls one trailer.",
		"Requirements",
		"Every truck of the fleet satisfies these requirements.",
		"<ol", "Load Limit", "Brake Distance", "Axle Count", "</ol>",
		"Safety",
		"Figures",
		"Traceability",
		"Oddities")

	// The Fleet section is named like the top-level package the view lives
	// in, so both Diagram blocks name the view from the global namespace. A
	// view inside a part def is reached through a usage of it the Document
	// declares; one inside another view through that view.
	// A filter, sort or collect before an Image transforms the diagrams as it
	// does the elements: Truck Figures keeps the two Truck.* diagrams sorted
	// by name, Other Figures the one diagram neither pattern excludes, Figure
	// Owners lists the diagrams' owners, and No Figures draws nothing once a
	// metaclass filter keeps no diagram.
	wantNote(t, r, "_st_nofig_image", migrate.Mapped,
		"it draws nothing: «FilterByMetaclasses» Fleet Viewpoints::No Figures Viewpoint::No Figures Method::Packages Only drops all the diagrams collected; an Image draws only diagrams")
	wantNote(t, r, "_st_odd_image", migrate.Mapped,
		"it draws nothing: the only element collected, the «Block» Class Fleet::Structure::Truck, is not a diagram; an Image draws only diagrams")
	brief := markdown(t, s, "'Fleet Documents'::'Fleet Brief Document'")
	wantInOrder(t, "Fleet Brief Markdown", brief,
		"# Fleet Brief", "## Figures", "*Truck Structure*", "```mermaid", "The truck and what it hauls",
		"## Fleet", "*Truck Structure*", "```mermaid", "The truck and what it hauls",
		"*Parts Method Flow*", "```mermaid", "action rendering (render Views::asInterconnectionDiagram, view def ActionFlowView)",
		"'Collect Owned Elements'<br>«action»", "'Filter By Metaclasses'<br>«action»", "'Sort By Name'<br>«action»",
		"*Truck Internals*", "```mermaid", "axles",
		"*Fleet Overview*", "```mermaid", "Requirements",
		"## Gallery", "*Figure: Inside the truck*", "```mermaid", "axles",
		"## Truck Figures", "*Truck Internals*", "```mermaid", "axles", "*Truck Structure*", "```mermaid", "Trailer",
		"## Other Figures", "*Fleet Overview*", "```mermaid", "Requirements",
		"## Figure Owners", "- Structure\n- Truck",
		"## No Figures")
	if strings.Count(brief, "```mermaid") != 9 {
		t.Errorf("Fleet Brief Markdown draws %d diagrams, want 9:\n%s", strings.Count(brief, "```mermaid"), brief)
	}
	if strings.Contains(brief, "rendered as textual notation") {
		t.Errorf("the activity diagram's figure is refused instead of drawn:\n%s", brief)
	}
	if body := markdownSection(brief, "## Truck Figures"); strings.Contains(body, "*Fleet Overview*") {
		t.Errorf("Truck Figures draws a diagram the name filter drops:\n%s", body)
	}
	if body := markdownSection(brief, "## Other Figures"); strings.Contains(body, "*Truck") {
		t.Errorf("Other Figures draws a diagram the name filter excludes:\n%s", body)
	}
	if body := markdownSection(brief, "## No Figures"); strings.Contains(body, "```mermaid") {
		t.Errorf("No Figures draws a diagram the metaclass filter drops:\n%s", body)
	}
	if strings.Contains(brief, "showCaptions is false") {
		t.Fatalf("a caption DocGen hides is rendered:\n%s", brief)
	}
}

// A view's own documentation opens its section, before what its method
// produces, as DocGen prints it: at every depth, tool HTML reduced to text,
// once when the same comment is also one of the view's collaborator paragraphs,
// and not at all for a view that has none.
func TestViewDocumentationOpensItsSection(t *testing.T) {
	r := migrateFixtureFile(t, "documents")
	notation := string(r.Notation)
	for id, target := range map[string]string{
		"_intro_doc":  "part 'Fleet Documents'::'Fleet Handbook Document'::Introduction::paragraph",
		"_safety_doc": "part 'Fleet Documents'::'Fleet Handbook Document'::Requirements::Safety::paragraph",
	} {
		var paragraphs []migrate.Entry
		for _, e := range entriesFor(r, id) {
			if e.Target == target {
				paragraphs = append(paragraphs, e)
			}
		}
		if len(paragraphs) != 1 || paragraphs[0].Verdict != migrate.Mapped || !strings.Contains(paragraphs[0].Note, "the documentation of the view Fleet Documents::") {
			t.Errorf("entries for %s = %+v, want one mapped entry -> %s noting the view's documentation", id, entriesFor(r, id), target)
		}
	}
	wantInOrder(t, "Introduction section", notationSection(notation, "Introduction"),
		`attribute redefines title = "Introduction";`,
		"part paragraph : DocumentQueries::Paragraph {",
		`attribute redefines text = "The fleet, in brief.";`,
		`attribute redefines caption = "Fleet Parts";`,
		"part 'paragraph 2' : DocumentQueries::Paragraph {",
		`attribute redefines text = "The parts of the fleet, by name.";`)
	wantInOrder(t, "Safety section", notationSection(notation, "Safety"),
		`attribute redefines title = "Safety";`,
		"part paragraph : DocumentQueries::Paragraph {",
		`attribute redefines text = "Safety comes first.";`,
		`attribute redefines caption = "Safety Requirements";`)
	for _, text := range []string{"The fleet, in brief.", "Safety comes first.", "Second note."} {
		if n := strings.Count(notation, `text = "`+text+`";`); n != 1 {
			t.Errorf("%q is written as %d paragraph(s), want 1:\n%s", text, n, notation)
		}
	}
	wantInOrder(t, "Requirements section", notationSection(notation, "Requirements"),
		`attribute redefines title = "Requirements";`,
		"part paragraph : DocumentQueries::Paragraph {",
		`attribute redefines text = "Every truck of the fleet satisfies these requirements.";`)
	if sec := notationSection(notation, "Figures"); strings.Count(sec, "DocumentQueries::Paragraph") != 1 {
		t.Errorf("Figures, a view with no documentation, does not hold its caption paragraph alone:\n%s", sec)
	}

	md := markdown(t, session(t, r), "'Fleet Documents'::'Fleet Handbook Document'")
	wantInOrder(t, "Fleet Handbook Markdown", md,
		"## Introduction", "The fleet, in brief.", "*Fleet Parts*", "The parts of the fleet, by name.",
		"## Requirements", "Every truck of the fleet satisfies these requirements.",
		"### Safety", "Safety comes first.", "| Brake Distance |",
		"## Notes", "First note.")
	if n := strings.Count(md, "Second note."); n != 1 {
		t.Errorf("a collaborator paragraph that is also its view's documentation is printed %d times, want 1:\n%s", n, md)
	}
}

// The DocGen collectors and filters that have a faithful query form follow the
// tool's semantics: FilterByDiagramType keeps or drops diagrams by the tool's
// presentation type, CollectThingsOnDiagram collects the elements a diagram
// shows, CollectByAssociation walks typed attributes of one aggregation kind to
// a depth, requirement columns read the id and text, and a filter over a
// stereotype with no v2 form stays refused.
func TestMigratedCollectorsAndFilters(t *testing.T) {
	r := migrateFixtureFile(t, "collectors")
	wantClean(t, "collectors.sysml", r)
	notation := string(r.Notation)

	// The presentation type, not the UML diagram kind, decides the filter:
	// the BDD is a "Class Diagram" in UML terms yet is kept as a block diagram,
	// and excluding it keeps the other two typed diagrams.
	wantInOrder(t, "Block Diagrams", notation,
		"part 'Block Diagrams' : DocumentQueries::Section {",
		`attribute redefines caption = "Crane Structure";`,
		"part 'Sketched Diagrams' : DocumentQueries::Section {",
		"part 'Retargeted Diagrams' : DocumentQueries::Section {",
		`attribute redefines caption = "Crane Structure";`,
		`attribute redefines caption = "Gantry Drive";`,
		"part 'All Diagrams' : DocumentQueries::Section {",
		`attribute redefines caption = "Crane Structure";`,
		"part 'No Diagrams' : DocumentQueries::Section {",
		"part 'Kin Diagrams' : DocumentQueries::Section {",
		`attribute redefines caption = "Sway Limits";`,
		`attribute redefines caption = "Yard Requirements";`,
		"part 'Other Diagrams' : DocumentQueries::Section {",
		`attribute redefines caption = "Crane Internals";`,
		`attribute redefines caption = "Yard Requirements";`,
		"part 'Internal Diagrams' : DocumentQueries::Section {",
		`attribute redefines caption = "Crane Internals";`,
		"part 'Parametric Diagrams' : DocumentQueries::Section {",
		`attribute redefines title = "Parametric Diagrams";`,
		"}")
	from, to := strings.Index(notation, "part 'Block Diagrams' : DocumentQueries::Section"), strings.Index(notation, "part 'Sketched Diagrams' : DocumentQueries::Section")
	if body := notation[from:to]; strings.Count(body, "DocumentQueries::Diagram") != 1 {
		t.Errorf("Block Diagrams draws diagrams the type filter drops:\n%s", body)
	}
	wantNote(t, r, "_st_bdds_image", migrate.Mapped, "")
	// An Image is refused even where a rejoined branch knows its diagrams: the
	// branch with an undecidable diagram type may add more.
	from, to = to, strings.Index(notation, "part 'Retargeted Diagrams' : DocumentQueries::Section")
	if body := notation[from:to]; strings.Contains(body, "DocumentQueries::Diagram") {
		t.Errorf("Sketched Diagrams draws the diagrams one branch knows while the other's are undecided:\n%s", body)
	}
	wantNote(t, r, "_st_sketch_image", migrate.Unmapped,
		"the diagrams it shows are not known: «FilterByDiagramType» Yard Viewpoints::Sketched Diagrams Viewpoint::Sketched Diagrams Method::Filter By Diagram Type keeps or drops the diagram 'Sketch', whose diagram type the archive does not record")
	// The doubt a filter leaves before a fork ends with the branches: each
	// names its own target, so the rejoined Image knows both its diagrams.
	from, to = to, strings.Index(notation, "part 'All Diagrams' : DocumentQueries::Section")
	if body := notation[from:to]; strings.Count(body, "DocumentQueries::Diagram") != 2 {
		t.Errorf("Retargeted Diagrams does not draw the two diagrams the branches target:\n%s", body)
	}
	wantOneNote(t, r, "_st_retarget_image", migrate.Mapped, "")
	for _, e := range entriesFor(r, "_st_retarget_image") {
		if e.Verdict == migrate.Unmapped {
			t.Errorf("Retargeted Diagrams keeps the doubt the branches replaced: %+v", e)
		}
	}
	// A filter naming no type decides without reading the types: excluding
	// keeps every diagram, the untyped one included, and including keeps none.
	from, to = to, strings.Index(notation, "part 'No Diagrams' : DocumentQueries::Section")
	if body := notation[from:to]; strings.Count(body, "DocumentQueries::Diagram") != 1 {
		t.Errorf("All Diagrams does not draw the one typed diagram an exclusion naming no type keeps:\n%s", body)
	}
	wantOneNote(t, r, "_st_all_image", migrate.Mapped, "")
	wantOneNote(t, r, "_st_all_image", migrate.Unmapped,
		"the diagram of unknown kind 'Sketch' is a view rendered as textual notation, which a document does not draw")
	from, to = to, strings.Index(notation, "part 'Kin Diagrams' : DocumentQueries::Section")
	if body := notation[from:to]; strings.Contains(body, "DocumentQueries::Diagram") || strings.Contains(body, "not migrated") {
		t.Errorf("No Diagrams draws or refuses where an inclusion naming no type knowingly keeps nothing:\n%s", body)
	}
	wantNote(t, r, "_st_noneof_image", migrate.Mapped,
		"it draws nothing: «FilterByDiagramType» Yard Viewpoints::No Diagrams Viewpoint::No Diagrams Method::Filter By Diagram Type drops all the diagrams collected: it names no diagram type; an Image draws only diagrams")
	// Owners stop short of the flattened root Model, so the owned elements
	// collected next are the requirement's package's, not every package's.
	from, to = to, strings.Index(notation, "part 'Other Diagrams' : DocumentQueries::Section")
	if body := notation[from:to]; strings.Count(body, "DocumentQueries::Diagram") != 2 {
		t.Errorf("Kin Diagrams draws diagrams owned beyond the requirement's package:\n%s", body)
	}
	wantNote(t, r, "_st_pars_image", migrate.Mapped,
		"it draws nothing: «FilterByDiagramType» Yard Viewpoints::Parametric Diagrams Viewpoint::Parametric Diagrams Method::Filter By Diagram Type drops all the diagrams collected: none is a SysML Parametric Diagram; an Image draws only diagrams")

	// Things on the diagram: the two blocks it shows are named; a nested
	// diagram, an unresolved reference, a connector end of an anonymous
	// connector and a comment written as its owner's doc are each left out
	// for their own reason.
	wantInOrder(t, "Shown Blocks query", notation,
		"calc def 'Yard Handbook Shown Blocks Rows'",
		`DocumentQueries::Named(qualifiedName = ("Structure::Hook", "Structure::Crane")),`,
		`property = "name",`,
		`properties = ("name", "documentation"))`)
	for _, note := range []string{
		"reads the 7 elements the tool lists as used on the SysML Block Definition Diagram 'Crane Structure', whose symbols are not serialized; the list need not be all it shows",
		"leaves out 2 elements shown on the SysML Block Definition Diagram 'Crane Structure' that the archive does not describe",
		"leaves out 1 element shown on the SysML Block Definition Diagram 'Crane Structure' that the migration does not write",
		"leaves out 1 element shown on the SysML Block Definition Diagram 'Crane Structure' written within the elements owning them, with no v2 element of their own",
	} {
		wantNote(t, r, "_st_shown_table", migrate.Approximated, note)
	}
	// A diagram whose contents the archive does not record leaves the whole
	// collection unknown: the elements the other diagrams show are not listed
	// as if they were all of them.
	wantNote(t, r, "_st_both_collect", migrate.Unmapped,
		"it collects what the diagram of unknown kind 'Sketch' shows, which the archive does not record")
	wantNote(t, r, "_st_both_table", migrate.Unmapped,
		"the elements it shows pass through «CollectThingsOnDiagram» Yard Viewpoints::Shown Everywhere Viewpoint::Shown Everywhere Method::Collect Things On Diagram is not migrated: it collects what the diagram of unknown kind 'Sketch' shows, which the archive does not record")
	if strings.Contains(notation, "calc def 'Yard Handbook Shown Everywhere Rows'") {
		t.Errorf("a collection over an unread diagram is spelled as the elements the other diagrams show:\n%s", notation)
	}

	// By association: composite parts to any depth stop at the cycle back to
	// Hook and skip the attribute whose type is missing; depth 1 keeps the
	// direct part only; shared aggregation keeps the cable; a block with no
	// composite part collects nothing.
	wantInOrder(t, "Parts queries", notation,
		"calc def 'Yard Handbook Parts Rows'",
		`DocumentQueries::Named(qualifiedName = ("Structure::Hook", "Structure::Latch")),`,
		"calc def 'Yard Handbook Direct Parts Rows'",
		`DocumentQueries::Named(qualifiedName = ("Structure::Hook")),`,
		"calc def 'Yard Handbook Shared Parts Rows'",
		`DocumentQueries::Named(qualifiedName = ("Structure::Cable")),`)
	// A type first reached at the depth limit through one root is walked
	// again when another root reaches it with depth to spare: the gantry's
	// trolley motor is two steps down, the winch's motor one, so its brake is
	// within the winch's two.
	wantInOrder(t, "Drive Parts query", notation,
		"calc def 'Yard Handbook Drive Parts Rows'",
		`DocumentQueries::Named(qualifiedName = ("Structure::Trolley", "Structure::Motor", "Structure::Brake")),`)
	if strings.Contains(notation, `"Structure::Operator"`) {
		t.Errorf("a plain reference is collected as a composite part:\n%s", notation)
	}
	wantOneNote(t, r, "_st_parts_list", migrate.Mapped,
		"it shows nothing: «CollectByAssociation» Yard Viewpoints::Parts Viewpoint::Parts Method::Collect By Association collects nothing: the only element collected, the «Block» Class Structure::Cable, has no typed attribute of composite aggregation")

	// Requirement id and text columns read the short name and the doc.
	wantInOrder(t, "Specification query", notation,
		"calc def 'Yard Handbook Requirements Specification Table Rows'",
		`type = ("RequirementDefinition")),`,
		`properties = ("shortName", "name", "documentation"))`)
	wantNote(t, r, "_st_todo_table", migrate.Unmapped,
		"the elements it shows pass through «FilterByStereotypes» Yard Viewpoints::To Do Viewpoint::To Do Method::Filter By Stereotypes is not migrated: no v2 metaclass stands for the elements of «TODO_Owner»")

	// A sort by documentation orders the source elements as OrderBy orders the
	// rows, so the diagrams collected from them follow: the crane and the
	// package sort by their doc comment, the requirement by its text rather
	// than its comment, and the gantry, with none, comes last either way;
	// diagrams sort by the doc their view writes from the diagram's own comment.
	wantInOrder(t, "Gantry Drive doc", notation,
		"view 'Gantry Drive' {", "doc /* The trolley the gantry carries. */", "expose Gantry;")
	wantInOrder(t, "Documented Owners query", notation,
		"calc def 'Yard Handbook Documented Owners Rows'",
		`property = "documentation",`, `direction = "ascending",`, `missing = "last",`, `multiple = "first"`)
	wantInOrder(t, "Documented Diagrams", notation,
		"part 'Documented Diagrams' : DocumentQueries::Section {",
		`attribute redefines caption = "Crane Internals";`,
		`attribute redefines caption = "Sway Limits";`,
		`attribute redefines caption = "Yard Requirements";`,
		`attribute redefines caption = "Gantry Drive";`,
		"part 'Reverse Documented Diagrams' : DocumentQueries::Section {",
		`attribute redefines caption = "Yard Requirements";`,
		`attribute redefines caption = "Sway Limits";`,
		`attribute redefines caption = "Crane Internals";`,
		`attribute redefines caption = "Gantry Drive";`,
		"part 'Diagrams By Documentation' : DocumentQueries::Section {",
		`attribute redefines caption = "Gantry Drive";`,
		`attribute redefines caption = "Crane Structure";`)
	// A sort with no query spelling breaks the chain for the Image after it as
	// for any other block: the diagrams current before it are not drawn as if
	// it had sorted them.
	wantNote(t, r, "_st_owner_sort", migrate.Unmapped, "no query property stands for the attribute Owner")
	wantNote(t, r, "_st_owner_image", migrate.Unmapped,
		"the diagrams it shows pass through «SortByAttribute» Yard Viewpoints::Owner Sorted Diagrams Viewpoint::Owner Sorted Diagrams Method::Sort By Owner is not migrated: no query property stands for the attribute Owner")

	// A name filter matches the v1 name, as DocGen does: the anonymous block the
	// write names `unnamed` does not pass the filter naming it, so the section
	// shows nothing, and no WhereName query over v2 names stands in its place.
	wantNote(t, r, "_st_drafts_list", migrate.Mapped, "it shows nothing: «FilterByNames» Yard Viewpoints::Unnamed Drafts Viewpoint::Unnamed Drafts Method::Filter By Names drops all 2 elements collected, none of them a diagram")
	wantNote(t, r, "_st_drafts_image", migrate.Mapped, "it draws nothing: «FilterByNames» Yard Viewpoints::Unnamed Drafts Viewpoint::Unnamed Drafts Method::Filter By Names drops all 2 elements collected, none of them a diagram; an Image draws only diagrams")
	if body := notationSection(notation, "Unnamed Drafts"); strings.Contains(body, "DocumentQueries::Diagram") || strings.Contains(body, "DocumentQueries::List") || strings.Contains(notation, `"^(?:unnamed)$"`) {
		t.Errorf("Unnamed Drafts shows the block the filter drops by v1 name:\n%s", body)
	}
	// Where an element's v1 and v2 names fall on different sides of the pattern,
	// the filter names the elements it keeps instead of spelling a WhereName
	// query, which would read the v2 name: the pattern matching the empty name
	// keeps the anonymous block and its named sibling alike.
	wantInOrder(t, "Optional Drafts", notation,
		"calc def 'Yard Handbook Bulleted List Rows'",
		`DocumentQueries::Named(qualifiedName = ("Drafts::unnamed", "Drafts::Sketchy"))`,
		"part 'Optional Drafts' : DocumentQueries::Section {",
		"calc items : 'Yard Handbook Bulleted List Rows';",
		`attribute redefines caption = "Overview";`,
		"ref redefines source = unnamed.Overview;",
		`attribute redefines caption = "Sketchy Overview";`,
		"ref redefines source = sketchy.'Sketchy Overview';")
	if body := notationSection(notation, "Optional Drafts"); strings.Count(body, "DocumentQueries::Diagram") != 2 {
		t.Errorf("Optional Drafts draws other than both blocks' diagrams:\n%s", body)
	}
	wantNote(t, r, "_st_optional_list", migrate.Approximated, "«FilterByNames» Yard Viewpoints::Optional Drafts Viewpoint::Optional Drafts Method::Filter By Names names the elements it keeps, since a WhereName query matches v2 names: the «Block» Class Drafts::<Class> is named unnamed in v2")
	wantOneNote(t, r, "_st_optional_image", migrate.Mapped, "")

	s := session(t, r)
	md := markdown(t, s, "'Yard Handbook'::'Yard Handbook Document'")
	wantInOrder(t, "Yard Handbook Markdown", md,
		"# Yard Handbook",
		"## Block Diagrams", "*Crane Structure*", "```mermaid",
		"## Sketched Diagrams",
		"## Retargeted Diagrams", "*Crane Structure*", "```mermaid", "*Gantry Drive*", "```mermaid",
		"## All Diagrams", "*Crane Structure*", "```mermaid",
		"## No Diagrams",
		"## Kin Diagrams", "*Sway Limits*", "```mermaid", "*Yard Requirements*", "```mermaid",
		"## Other Diagrams", "*Crane Internals*", "```mermaid", "*Yard Requirements*", "```mermaid",
		"## Internal Diagrams", "*Crane Internals*", "```mermaid",
		"## Parametric Diagrams",
		"## Shown On The Structure Diagram",
		"| name | documentation |",
		"| Crane | Lifts containers off the quay. |",
		"| Hook | Holds the spreader. |",
		"## Shown On Both Diagrams",
		"## Drive Parts", "- Trolley\n- Motor\n- Brake",
		"## Crane Parts", "- Hook\n- Latch",
		"## Direct Crane Parts", "- Hook",
		"## Shared Crane Parts", "- Cable",
		"## Cable Parts",
		"## To Do",
		"## Specification",
		"| shortName | name | documentation |",
		"| Y-1 | Lift | The crane lifts a loaded container. |",
		"| Y-2 | Sway | The load sways less than one degree. |",
		"## Documented Diagrams", "- Crane\n- Sway\n- Needs\n- Gantry",
		"*Crane Internals*", "```mermaid", "*Sway Limits*", "```mermaid",
		"*Yard Requirements*", "```mermaid", "*Gantry Drive*", "```mermaid",
		"## Reverse Documented Diagrams",
		"*Yard Requirements*", "```mermaid", "*Sway Limits*", "```mermaid",
		"*Crane Internals*", "```mermaid", "*Gantry Drive*", "```mermaid",
		"## Diagrams By Documentation", "*Gantry Drive*", "```mermaid", "*Crane Structure*", "```mermaid",
		"## Owner Sorted Diagrams",
		"## Unnamed Drafts",
		"## Optional Drafts", "- unnamed\n- Sketchy", "*Overview*", "```mermaid", "*Sketchy Overview*", "```mermaid")
	if n := strings.Count(md, "```mermaid"); n != 21 {
		t.Errorf("Yard Handbook Markdown draws %d diagrams, want 21:\n%s", n, md)
	}
	for _, heading := range []string{"## Sketched Diagrams", "## No Diagrams", "## Parametric Diagrams", "## Shown On Both Diagrams", "## Cable Parts", "## To Do", "## Owner Sorted Diagrams", "## Unnamed Drafts"} {
		if body := markdownSection(md, heading); strings.Contains(body, "```mermaid") || strings.Contains(body, "|") || strings.Contains(body, "\n- ") {
			t.Errorf("%s shows content DocGen has nothing for:\n%s", heading, body)
		}
	}
}

// Only the tool's own profile namespaces define tables: a user stereotype named
// InstanceTable, TableStructure or Document is ordinary metadata, applications
// of look-alike stereotypes under other URIs stay comments, and an exact-profile
// table whose serialization is malformed is refused with the fault named, its
// view and its independent siblings still written.
func TestTableHomonymsAndMalformedTables(t *testing.T) {
	r := migrateFixtureFile(t, "table_homonyms")
	wantClean(t, "table_homonyms.sysml", r)
	notation := string(r.Notation)

	wantInOrder(t, "user profile", notation,
		"metadata def TableStructure {", "metadata def InstanceTable {", "metadata def Document;",
		"part def Catalog {", "@'Shop Profile'::InstanceTable {", `scope = "Shop";`,
		"part def Ledger {", "@'Shop Profile'::TableStructure {", "rows = 12;",
		"part def Report {", "@'Shop Profile'::Document;", "/* applied stereotype «Document» */")
	wantNote(t, r, "_blk_report", migrate.Mapped,
		"«Document» from http://www.magicdraw.com/schemas/manual/Document_Profile_Custom.xmi is applied from a profile the document does not define")
	for _, name := range []string{"Catalog Table", "Custom Table"} {
		if strings.Contains(notation, "'"+name+" Rows'") || strings.Contains(notation, "'"+name+" Document'") {
			t.Errorf("the look-alike %s on a non-profile URI lowered to a query:\n%s", name, notation)
		}
	}
	if n := strings.Count(notation, ":> DocumentQueries::Document {"); n != 1 {
		t.Errorf("%d Documents written, want only the valid Catalog Map:\n%s", n, notation)
	}

	refusals := map[string]string{
		"_tbl_dangling":      "the scope _nowhere resolves to no element",
		"_tbl_ambiguous":     "the scope _shared names 2 module elements (http://example.com/modules/Warehouse.xmi#_shared, http://example.com/modules/Storefront.xmi#_shared)",
		"_tbl_bad_sort":      `sort "IColumn:_prop_price^Sideways": not in the form <column>^Asc|Desc; sort "price": not in the form <column>^Asc|Desc`,
		"_tbl_no_classifier": "the instance table names no classifier",
		"_tbl_ghost_column":  "the column IColumn:_no_such_property names no property of the document",
		"_tbl_no_diagram":    "base_Diagram _no_such_diagram names no diagram of the document",
		"_mx_broken":         "the unnamed criterion is malformed: not well-formed XML: xmi: XML syntax error on line 4: unexpected EOF",
		"_mx_orphan":         "MatrixFilter: no filter application names the diagram",
		"_map_deep":          `depth "deep": not a non-negative integer`,
	}
	for id, why := range refusals {
		wantNote(t, r, id, migrate.Unmapped, why)
	}
	wantInOrder(t, "refused tables", notation,
		"view 'Dangling Scope' {", "/* not migrated: «InstanceTable» 'Dangling Scope' — the scope _nowhere resolves to no element */",
		"view 'Ambiguous Scope' {", "/* not migrated: «InstanceTable» 'Ambiguous Scope' — the scope _shared names 2 module elements (http://example.com/modules/Warehouse.xmi#_shared, http://example.com/modules/Storefront.xmi#_shared) */",
		"view 'Broken Matrix' {", "expose Catalog;", "/* not migrated: «DependencyMatrix» 'Broken Matrix' — the unnamed criterion is malformed",
		"view 'Deep Map' {", "/* not migrated: «RelationMap» 'Deep Map' — depth \"deep\"",
		"view 'Catalog Map' {", "expose 'Catalog Map Document';",
		"calc def 'Catalog Map Rows' :> DocumentQueries::Query {",
		`relationshipKind = "specialization"`, `direction = "incoming"`, "maxDepth = 1",
		"part def 'Catalog Map Document' :> DocumentQueries::Document {")
	wantNote(t, r, "_map_catalog", migrate.Mapped, "written as a Document holding a Table over the query 'Catalog Map Rows'")

	s := session(t, r)
	wantInOrder(t, "Catalog Map rows", rows(t, s, "Shop::'Catalog Map Rows'"),
		"returned 2 rows", "Shop::SeasonalCatalog", `@type = "PartDefinition"`, "Shop::c1")
	wantInOrder(t, "Catalog Map Markdown", markdown(t, s, "Shop::'Catalog Map Document'"),
		"# Catalog Map", "| qualifiedName | @type |", "| Shop::SeasonalCatalog | PartDefinition |", "| Shop::c1 | PartDefinition |")
}

// A stereotype filter tells stereotypes apart by the profile that defines
// them, not by their local name: two «Critical» of different profiles differ,
// a derived stereotype of the same profile matches, and an application of a
// profile the document does not define leaves the filter undecided.
func TestMigratedStereotypeFilters(t *testing.T) {
	r := migrateFixtureFile(t, "stereotype_filters")
	wantClean(t, "stereotype_filters.sysml", r)
	notation := string(r.Notation)

	section := func(name, next string) string {
		from := strings.Index(notation, "part '"+name+"' : DocumentQueries::Section")
		to := strings.Index(notation, "part '"+next+"' : DocumentQueries::Section")
		if from < 0 || to < from {
			t.Fatalf("no section %q before %q:\n%s", name, next, notation)
		}
		return notation[from:to]
	}
	// The Pump is «Safety Profile::Critical», the Valve «Cost Profile::Critical»,
	// the Tank «Safety Profile::Vital» deriving from the former.
	if body := section("Exact Critical Layouts", "Critical Figures"); strings.Count(body, "DocumentQueries::Diagram") != 1 || !strings.Contains(body, `caption = "Pump Layout"`) {
		t.Errorf("Exact Critical Layouts draws other than the Pump's diagram:\n%s", body)
	}
	if body := section("Critical Layouts", "Satisfier Figures"); strings.Count(body, "DocumentQueries::Diagram") != 2 ||
		!strings.Contains(body, `caption = "Pump Layout"`) || !strings.Contains(body, `caption = "Tank Layout"`) {
		t.Errorf("Critical Layouts draws other than the Pump's and the Tank's diagrams:\n%s", body)
	}
	if body := section("Uncritical Figures", "Critical Layouts"); strings.Count(body, "DocumentQueries::Diagram") != 4 {
		t.Errorf("Uncritical Figures does not keep every diagram, none being «Critical»:\n%s", body)
	}
	wantInOrder(t, "Critical Items query", notation,
		"calc def 'Depot Handbook Critical Items Rows'",
		`'metadata' = ("Safety Profile::Critical")`)

	// The Hose's «Vendor::Critical» is of no profile the document defines, so
	// whether it is the «Critical» wanted is not known before the query runs.
	undecided := "«FilterByStereotypes» Depot Viewpoints::Owned Figures Viewpoint::Owned Figures Method::Filter By Stereotypes keeps or drops the «Block» Class Spares::Hose, which cannot be told before the query runs"
	wantNote(t, r, "_st_owned_image", migrate.Unmapped, "the diagrams it shows are not known: "+undecided)
	// A diagram-type filter drops whatever the undecided holders are, since
	// none is a diagram: the Image after it knowingly draws nothing.
	wantNote(t, r, "_st_figures_image", migrate.Mapped,
		"it draws nothing: «FilterByDiagramType» Depot Viewpoints::Critical Figures Viewpoint::Critical Figures Method::Filter By Diagram Type drops all the elements collected, whichever they are, and it keeps only diagrams: "+
			strings.Replace(undecided, "Owned Figures Viewpoint::Owned Figures Method", "Critical Figures Viewpoint::Critical Figures Method", 1)+"; an Image draws only diagrams")
	if body := section("Critical Figures", "Uncritical Figures"); strings.Contains(body, "DocumentQueries::Diagram") {
		t.Errorf("Critical Figures draws a diagram:\n%s", body)
	}

	// Elements found by following relationships are known only when the
	// query runs, yet none is a diagram: a diagram-type filter leaves nothing,
	// while collecting what they own may reach diagrams, so an Image after
	// that is refused.
	wantNote(t, r, "_st_sat_image", migrate.Mapped,
		"it draws nothing: «FilterByDiagramType» Depot Viewpoints::Satisfier Figures Viewpoint::Satisfier Figures Method::Filter By Diagram Type drops all the elements collected, whichever they are, and it keeps only diagrams: «CollectByDirectedRelationshipStereotypes» Depot Viewpoints::Satisfier Figures Viewpoint::Satisfier Figures Method::Collect Satisfiers follows relationships to elements only the query finds; an Image draws only diagrams")
	wantNote(t, r, "_st_satown_image", migrate.Unmapped,
		"the diagrams it shows are not known: «CollectByDirectedRelationshipStereotypes» Depot Viewpoints::Satisfier Layouts Viewpoint::Satisfier Layouts Method::Collect Satisfiers follows relationships to elements only the query finds")

	s := session(t, r)
	wantInOrder(t, "Critical Items rows", rows(t, s, "'Depot Handbook'::'Depot Handbook Critical Items Rows'"),
		"returned 2 rows", "Pump", "Tank")
}
