package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
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
// and a refused node leaves its enclosing section in place.
func TestMigratedDocumentsRender(t *testing.T) {
	s := session(t, migrateFixtureFile(t, "documents"))

	md := markdown(t, s, "'Fleet Documents'::'Fleet Handbook Document'")
	wantInOrder(t, "Fleet Handbook Markdown", md,
		"# Fleet Handbook",
		"## Introduction",
		"| name | qualifiedName | documentation | Payload |",
		"| Axle | Fleet::Structure::Axle |  |  |",
		"| Trailer | Fleet::Structure::Trailer | Carries the load. |  |",
		"| Truck | Fleet::Structure::Truck | Hauls one trailer. |  |",
		"## Requirements",
		"Every truck of the fleet satisfies these requirements.",
		"1. Load Limit", "2. Brake Distance", "3. Axle Count",
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
		"## Notes")
	if strings.Contains(md, "Axle Count | ") {
		t.Fatalf("the Safety table lists a requirement outside the Safety filter:\n%s", md)
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

	brief := markdown(t, s, "'Fleet Documents'::'Fleet Brief Document'")
	wantInOrder(t, "Fleet Brief Markdown", brief,
		"# Fleet Brief", "## Figures", "*The truck and what it hauls*", "```mermaid")
}
