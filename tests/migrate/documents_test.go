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
	s := session(t, r)

	md := markdown(t, s, "'Fleet Documents'::'Fleet Handbook Document'")
	wantInOrder(t, "Fleet Handbook Markdown", md,
		"# Fleet Handbook",
		"## Introduction",
		"| name | qualifiedName | documentation | Payload | name 2 |",
		"| Axle | Fleet::Structure::Axle |  |  |  |",
		"| Trailer | Fleet::Structure::Trailer | Carries the load. |  |  |",
		"| Truck | Fleet::Structure::Truck | Hauls one trailer. |  |  |",
		"## Requirements",
		"Every truck of the fleet satisfies these requirements.",
		"1. Load Limit\n2. Brake Distance\n3. Axle Count",
		"The payload stays under the axle rating. A loaded truck stops within the legal distance. A truck has two axles.",
		"### Safety",
		"| Brake Distance | A loaded truck stops within the legal distance. |",
		"## Figures",
		"*The truck and what it hauls*",
		"```mermaid",
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
	brief := markdown(t, s, "'Fleet Documents'::'Fleet Brief Document'")
	wantInOrder(t, "Fleet Brief Markdown", brief,
		"# Fleet Brief", "## Figures", "*The truck and what it hauls*", "```mermaid",
		"## Fleet", "*The truck and what it hauls*", "```mermaid",
		"*Truck Internals*", "```mermaid", "axles",
		"*Fleet Overview*", "```mermaid", "Requirements")
	if strings.Count(brief, "```mermaid") != 4 {
		t.Errorf("Fleet Brief Markdown draws %d diagrams, want 4:\n%s", strings.Count(brief, "```mermaid"), brief)
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
