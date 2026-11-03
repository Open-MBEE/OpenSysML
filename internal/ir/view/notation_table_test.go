package view

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// notationTableRow is one row of the docs' notation table: a §8.2.3 production,
// the edge kinds drawn with its glyph, and the glyph each form writes.
type notationTableRow struct {
	production, dot, plantuml, mermaid, d2, drawio, canvas, status string
	kinds                                                          []string
}

// notationTableKinds matches the backticked names of a table cell.
var notationTableKinds = regexp.MustCompile("`([^`]*)`")

// readNotationTable reads the rows of the notation table, the cells unescaped.
func readNotationTable(t *testing.T) []notationTableRow {
	t.Helper()
	raw, err := os.ReadFile("../../../docs/project/view-rendering-forms.md")
	if err != nil {
		t.Fatalf("read the rendering forms document: %v", err)
	}
	_, section, found := strings.Cut(string(raw), "\n### Notation table\n")
	if !found {
		t.Fatal("docs/project/view-rendering-forms.md has no notation table")
	}
	section, _, _ = strings.Cut(section, "\n### ")
	var rows []notationTableRow
	for _, line := range strings.Split(section, "\n") {
		if !strings.HasPrefix(line, "| `") && !strings.HasPrefix(line, "| objective") {
			continue
		}
		cells := strings.Split(strings.ReplaceAll(strings.Trim(line, "|"), `\|`, "\x00"), "|")
		if len(cells) != 10 {
			t.Fatalf("notation table row has %d cells, want 10: %s", len(cells), line)
		}
		for i, cell := range cells {
			cells[i] = strings.ReplaceAll(strings.TrimSpace(cell), "\x00", "|")
		}
		row := notationTableRow{production: cells[0], dot: cells[2], plantuml: cells[3], mermaid: cells[4], d2: cells[5], drawio: cells[6], canvas: cells[7], status: cells[8]}
		for _, m := range notationTableKinds.FindAllStringSubmatch(cells[1], -1) {
			row.kinds = append(row.kinds, m[1])
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		t.Fatal("the notation table has no rows")
	}
	return rows
}

// unticked is a cell's text without its code marks.
func unticked(cell string) string { return strings.Trim(cell, "`") }

// TestNotationTableMatchesTheWriters holds the DOT, PlantUML, Mermaid, D2 and draw.io
// writers to the notation table of docs/project/view-rendering-forms.md: every
// edge kind is in exactly one row, every row names a kind that exists and a
// status the table defines, and the glyph each writer draws for the kind is
// the one its cell states. The canvas column is checked by the extension's
// canvas test, which reads the same table.
func TestNotationTableMatchesTheWriters(t *testing.T) {
	byName := map[string]EdgeKind{}
	for kind := EdgeConnection; kind.String() != "edge"; kind++ {
		byName[kind.String()] = kind
	}
	seen := map[EdgeKind]string{}
	for _, row := range readNotationTable(t) {
		switch row.status {
		case "conformant", "approximate", "differs from pilot":
		default:
			t.Errorf("%s: status %q is none the table defines", row.production, row.status)
		}
		if len(row.kinds) == 0 {
			t.Errorf("%s: names no edge kind", row.production)
		}
		for _, name := range row.kinds {
			kind, ok := byName[name]
			if !ok {
				t.Errorf("%s: no edge kind is named %q", row.production, name)
				continue
			}
			if other, dup := seen[kind]; dup {
				t.Errorf("%s: edge kind %s is also in the %s row", row.production, name, other)
			}
			seen[kind] = row.production
			checkNotationRow(t, row, kind)
		}
	}
	for kind := EdgeConnection; kind.String() != "edge"; kind++ {
		if _, ok := seen[kind]; !ok {
			t.Errorf("edge kind %s is in no row of the notation table", kind)
		}
	}
}

// checkNotationRow checks one edge kind against its row: the DOT attributes
// after the label, the PlantUML and Mermaid arrows, and the D2 arrow and class.
func checkNotationRow(t *testing.T, row notationTableRow, kind EdgeKind) {
	t.Helper()
	rendering := &Rendering{View: "V", Kind: KindAction,
		Roots: []*Node{{ID: "a", Kind: "action", Name: "a"}, {ID: "b", Kind: "action", Name: "b"}},
		Edges: []Edge{{From: "a", To: "b", Label: "k", Kind: kind}}}
	dot, err := rendering.DOTWith(fullNotation)
	if err != nil {
		t.Fatalf("%s: DOT: %v", kind, err)
	}
	wantDOT := `"a" -> "b" [label="k"];`
	if row.dot != "*(default)*" {
		wantDOT = `"a" -> "b" [label="k", ` + unticked(row.dot) + "];"
	}
	if !strings.Contains(dot, wantDOT) {
		t.Errorf("%s (%s): DOT lacks %q:\n%s", row.production, kind, wantDOT, dot)
	}
	if got, want := plantumlArrow(KindAction, kind), unticked(row.plantuml); got != want {
		t.Errorf("%s (%s): PlantUML arrow %q, table says %q", row.production, kind, got, want)
	}
	if got, want := mermaidArrow(kind), unticked(row.mermaid); got != want {
		t.Errorf("%s (%s): Mermaid arrow %q, table says %q", row.production, kind, got, want)
	}
	arrow, class, _ := strings.Cut(unticked(row.d2), " ")
	d2, err := rendering.D2With(fullNotation)
	if err != nil {
		t.Fatalf("%s: D2: %v", kind, err)
	}
	if want := "\na " + arrow + ` b: "k" { class: ` + class + " }\n"; !strings.Contains(d2, want) {
		t.Errorf("%s (%s): D2 lacks %q:\n%s", row.production, kind, want, d2)
	}
	if got, want := drawioEdgeStyle(kind), unticked(row.drawio); got != want {
		t.Errorf("%s (%s): draw.io style %q, table says %q", row.production, kind, got, want)
	}
	drawio, err := rendering.DrawioWith(fullNotation)
	if err != nil {
		t.Fatalf("%s: draw.io: %v", kind, err)
	}
	if want := ";" + unticked(row.drawio) + `;" edge="1"`; !strings.Contains(drawio, want) {
		t.Errorf("%s (%s): draw.io lacks %q:\n%s", row.production, kind, want, drawio)
	}
}
