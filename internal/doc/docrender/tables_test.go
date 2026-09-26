package docrender

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const sizedReport = "Observatory::SizedReport"

func sizedReportPath() string { return filepath.Join("testdata", "sized_report.sysml") }

// TestHTMLTableColumnWidths locks that a table with stated widths is marked
// sized and carries one <col> per column: the stated width as data and the
// proportional share as style, an automatic column taking the mean stated width.
func TestHTMLTableColumnWidths(t *testing.T) {
	got := renderFixtureHTML(t, sizedReportPath(), sizedReport, HTMLOptions{Fragment: true})
	for _, want := range []string{
		`<table class="sysml-table sysml-table-sized" data-content="table" data-name="parts"`,
		"<colgroup>\n<col data-width=\"774\" style=\"width: 52.7%\">\n<col data-width=\"206\" style=\"width: 14.0%\">\n<col style=\"width: 33.3%\">\n</colgroup>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("HTML lacks %q:\n%s", want, got)
		}
	}
	if strings.Count(got, "<colgroup>") != 1 {
		t.Fatalf("an unsized table carries a column group:\n%s", got)
	}
	if !strings.Contains(got, `<table class="sysml-table" data-content="table" data-name="readings"`) {
		t.Fatalf("the unsized table is marked sized:\n%s", got)
	}
}

// TestHTMLTableRowDepth locks that a nested row states its depth as data and
// as the --sysml-depth property, its first cell opening with the indent, and
// that a top-level row states none.
func TestHTMLTableRowDepth(t *testing.T) {
	got := renderFixtureHTML(t, sizedReportPath(), sizedReport, HTMLOptions{Fragment: true})
	for _, want := range []string{
		`<tr class="sysml-row" data-element="Observatory::telescope::optics" data-element-kind="partUsage">` + "\n" +
			`<td class="sysml-cell" data-column="name" data-value-kind="string"><span class="sysml-value"`,
		`<tr class="sysml-row" data-element="Observatory::telescope::optics::cell" data-element-kind="partUsage" data-depth="1" style="--sysml-depth: 1">` + "\n" +
			`<td class="sysml-cell" data-column="name" data-value-kind="string"><span class="sysml-indent"></span><span class="sysml-value"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("HTML lacks %q:\n%s", want, got)
		}
	}
	if strings.Count(got, "sysml-indent") != 1 {
		t.Fatalf("indent count = %d, want the one nested row:\n%s", strings.Count(got, "sysml-indent"), got)
	}
}

// TestHTMLTableContinuation locks the split of a table wider than
// TableColumns: continuation tables each repeat the first column, carry a
// numbered id and a "(continued)" caption, and the option leaves a narrower
// table whole.
func TestHTMLTableContinuation(t *testing.T) {
	got := renderFixtureHTML(t, sizedReportPath(), sizedReport, HTMLOptions{Fragment: true, TableColumns: 8, NumberFigures: true})
	if n := strings.Count(got, `data-name="readings"`); n != 3 {
		t.Fatalf("readings tables = %d, want 3:\n%s", n, got)
	}
	if n := strings.Count(got, "sysml-table-continued"); n != 2 {
		t.Fatalf("continuation tables = %d, want 2:\n%s", n, got)
	}
	for _, want := range []string{
		`<table class="sysml-table" data-content="table" data-name="readings"`,
		`<table class="sysml-table sysml-table-continued" data-content="table" data-name="readings"`,
		`<caption class="sysml-caption"><span class="sysml-caption-number">Table 2.</span> Readings</caption>`,
		`<caption class="sysml-caption"><span class="sysml-caption-number">Table 2.</span> Readings (continued)</caption>`,
		"<thead>\n<tr>\n<th scope=\"col\" data-column=\"name\">name</th>\n<th scope=\"col\" data-column=\"h\">h</th>",
		"<thead>\n<tr>\n<th scope=\"col\" data-column=\"name\">name</th>\n<th scope=\"col\" data-column=\"o\">o</th>\n<th scope=\"col\" data-column=\"p\">p</th>\n<th scope=\"col\" data-column=\"q\">q</th>\n<th scope=\"col\" data-column=\"r\">r</th>\n<th scope=\"col\" data-column=\"s\">s</th>\n</tr>\n</thead>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("HTML lacks %q:\n%s", want, got)
		}
	}
	if n := strings.Count(got, `data-column="name" data-value-kind="string"><span class="sysml-value" data-value-kind="string">sample</span>`); n != 3 {
		t.Fatalf("first column repeated %d times, want 3", n)
	}
	if n := strings.Count(got, "Table 3."); n != 0 {
		t.Fatalf("continuation tables took caption numbers of their own:\n%s", got)
	}
	if n := strings.Count(got, `data-name="parts"`); n != 1 {
		t.Fatalf("the three-column table was split:\n%s", got)
	}
	whole := renderFixtureHTML(t, sizedReportPath(), sizedReport, HTMLOptions{Fragment: true})
	if strings.Contains(whole, "continued") || strings.Count(whole, `data-name="readings"`) != 1 {
		t.Fatalf("the default split a table:\n%s", whole)
	}
}

// TestTableParts locks the column index sets a table is split into.
func TestTableParts(t *testing.T) {
	cases := []struct {
		columns, limit int
		want           [][]int
	}{
		{3, 0, [][]int{{0, 1, 2}}},
		{3, 3, [][]int{{0, 1, 2}}},
		{3, 1, [][]int{{0, 1, 2}}},
		{0, 4, [][]int{{}}},
		{5, 3, [][]int{{0, 1, 2}, {0, 3, 4}}},
		{6, 3, [][]int{{0, 1, 2}, {0, 3, 4}, {0, 5}}},
	}
	for _, tc := range cases {
		if got := tableParts(tc.columns, tc.limit); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("tableParts(%d, %d) = %v, want %v", tc.columns, tc.limit, got, tc.want)
		}
	}
}

// TestMarkdownTableRowDepth locks that a nested row's first cell opens with
// the nesting marker while its name is written as stated.
func TestMarkdownTableRowDepth(t *testing.T) {
	got := renderFixtureDocument(t, sizedReportPath(), sizedReport)
	for _, want := range []string{
		"| optics | The primary mirror assembly. | 8.5 |",
		"| ↳ cell | Holds the mirror. | 2 |",
		"| mount | Points the telescope. | 15 |",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("Markdown lacks %q:\n%s", want, got)
		}
	}
	if marker := nestingMarker(3); marker != "&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;↳ " {
		t.Fatalf("nestingMarker(3) = %q", marker)
	}
}

// TestMarkdownTableContinuation locks the Markdown backend's split under
// TableColumns: continuation pipe tables each repeat the first column under
// the caption with its continued suffix, and the option leaves a narrower
// table whole.
func TestMarkdownTableContinuation(t *testing.T) {
	got, err := Markdown(fixtureDocument(t, sizedReportPath(), sizedReport), MarkdownOptions{TableColumns: 8, NumberFigures: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"*Table 2. Readings*\n\n| name | a | b | c | d | e | f | g |\n",
		"*Table 2. Readings (continued)*\n\n| name | h | i | j | k | l | m | n |\n",
		"*Table 2. Readings (continued)*\n\n| name | o | p | q | r | s |\n",
		"| sample | 1 | 2 | 3 | 4 | 5 | 6 | 7 |",
		"| sample | 8 | 9 | 10 | 11 | 12 | 13 | 14 |",
		"| sample | 15 | 16 | 17 | 18 | 19 |",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("Markdown lacks %q:\n%s", want, got)
		}
	}
	if n := strings.Count(got, "(continued)"); n != 2 {
		t.Fatalf("continuation captions = %d, want 2:\n%s", n, got)
	}
	if strings.Contains(got, "Table 3.") {
		t.Fatalf("continuation tables took caption numbers of their own:\n%s", got)
	}
	whole := renderFixtureDocument(t, sizedReportPath(), sizedReport)
	if strings.Contains(whole, "continued") {
		t.Fatalf("the default split a table:\n%s", whole)
	}
}
