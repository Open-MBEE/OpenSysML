package docrender

import (
	"errors"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
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

// TestHTMLTableColumnLabels locks that a column's stated label heads it while
// its name stays the column's data, and that an unlabelled column is headed
// by its name, in HTML and Markdown alike.
func TestHTMLTableColumnLabels(t *testing.T) {
	got := renderFixtureHTML(t, sizedReportPath(), sizedReport, HTMLOptions{Fragment: true})
	want := "<thead>\n<tr>\n<th scope=\"col\" data-column=\"name\">Item</th>\n" +
		"<th scope=\"col\" data-column=\"documentation\">Description</th>\n" +
		"<th scope=\"col\" data-column=\"mass\">mass</th>\n</tr>\n</thead>"
	if !strings.Contains(got, want) {
		t.Fatalf("HTML lacks %q:\n%s", want, got)
	}
	if !strings.Contains(got, `<td class="sysml-cell" data-column="documentation" data-value-kind="string">`) {
		t.Fatalf("cells are keyed by the label, not the column name:\n%s", got)
	}
	markdown := renderFixtureDocument(t, sizedReportPath(), sizedReport)
	if !strings.Contains(markdown, "| Item | Description | mass |\n| --- | --- | --- |\n| optics |") {
		t.Fatalf("Markdown lacks the labelled header:\n%s", markdown)
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
		`<table class="sysml-table sysml-table-wide sysml-table-split" data-content="table" data-name="readings"`,
		`<table class="sysml-table sysml-table-wide sysml-table-split sysml-table-continued" data-content="table" data-name="readings"`,
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
	if n := strings.Count(got, "sysml-table-split"); n != 3 {
		t.Fatalf("parts marked split = %d, want the three readings parts:\n%s", n, got)
	}
	whole := renderFixtureHTML(t, sizedReportPath(), sizedReport, HTMLOptions{Fragment: true})
	if strings.Contains(whole, "continued") || strings.Count(whole, `data-name="readings"`) != 1 || strings.Contains(whole, "sysml-table-split") {
		t.Fatalf("the default split a table:\n%s", whole)
	}
}

// TestTableContinuationUncaptioned locks that the continuations of a table
// without a caption print no caption of their own in HTML or Markdown.
func TestTableContinuationUncaptioned(t *testing.T) {
	const uncaptioned = "Observatory::UncaptionedReport"
	got := renderFixtureHTML(t, sizedReportPath(), uncaptioned, HTMLOptions{Fragment: true, TableColumns: 8})
	if n := strings.Count(got, "sysml-table-continued"); n != 2 {
		t.Fatalf("continuation tables = %d, want 2:\n%s", n, got)
	}
	if strings.Contains(got, "continued)") || strings.Contains(got, "<caption") {
		t.Fatalf("an uncaptioned table's continuations were captioned:\n%s", got)
	}
	md, err := Markdown(fixtureDocument(t, sizedReportPath(), uncaptioned), MarkdownOptions{TableColumns: 8})
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(md, "| name |"); n != 3 {
		t.Fatalf("Markdown readings parts = %d, want 3:\n%s", n, md)
	}
	if strings.Contains(md, "continued") || strings.Contains(md, "*Table") {
		t.Fatalf("an uncaptioned table's Markdown continuations were captioned:\n%s", md)
	}
}

// TestWidthSharesOverlongHeadings locks that headings needing more than the
// line between them share it in proportion to their needs, summing to one.
func TestWidthSharesOverlongHeadings(t *testing.T) {
	columns := []queryexec.Column{queryexec.Column{}.WithLabel("name"), queryexec.Column{}.WithLabel("postSegmentExchangeTimeLimit")}
	const measure = 20
	minima := []float64{
		max(float64(headingNeed(columns[0]))/measure, firstColumnMinShare),
		float64(headingNeed(columns[1])) / measure,
	}
	for _, widths := range [][]float64{{1, 1}, {774, 105}} {
		shares := widthShares(columns, widths, measure)
		if total := sum(shares); math.Abs(total-1) > 1e-9 {
			t.Fatalf("shares %v of widths %v sum to %v", shares, widths, total)
		}
		if math.Abs(shares[0]-minima[0]/(minima[0]+minima[1])) > 1e-9 {
			t.Fatalf("shares of widths %v = %v, want the headings' needs in proportion", widths, shares)
		}
	}
	if got := fitShares([]float64{0.5, 0.5}, []float64{0.2, 0.2}); !reflect.DeepEqual(got, []float64{0.5, 0.5}) {
		t.Fatalf("fitting shares changed to %v", got)
	}
}

// TestHTMLTableMeasureSplit locks the split by measure: the readings table's
// single-letter headings all fit a line of 320 characters — the three-column
// parts table has half of it — a line of 40 takes as many as head unbroken,
// and a sized table's column group widens a column to what its heading needs.
func TestHTMLTableMeasureSplit(t *testing.T) {
	got := renderFixtureHTML(t, sizedReportPath(), sizedReport, HTMLOptions{Fragment: true, TableMeasure: 320})
	if strings.Contains(got, "continued") || strings.Contains(got, "sysml-table-wide") {
		t.Fatalf("a table whose headings fit the line was split or set wide:\n%s", got)
	}
	if !strings.Contains(got, "<colgroup>\n<col data-width=\"774\" style=\"width: 52.7%\">") {
		t.Fatalf("headings shorter than their shares changed the shares:\n%s", got)
	}
	got = renderFixtureHTML(t, sizedReportPath(), sizedReport, HTMLOptions{Fragment: true, TableMeasure: 40})
	// name needs 7 of 40 characters and each letter heading 4: eight letters
	// fit beside name (39), a ninth does not.
	if n := strings.Count(got, `data-name="readings"`); n != 3 {
		t.Fatalf("readings parts = %d, want 3:\n%s", n, got)
	}
	if !strings.Contains(got, "<th scope=\"col\" data-column=\"name\">name</th>\n<th scope=\"col\" data-column=\"i\">i</th>") {
		t.Fatalf("the continuation does not start at the ninth letter:\n%s", got)
	}
	// Description needs 14 of 40 characters, over its 14.0% share, so it is
	// pinned at 35% and name and mass split the rest at 774 : 490.
	if !strings.Contains(got, "<colgroup>\n<col data-width=\"774\" style=\"width: 39.8%\">\n<col data-width=\"206\" style=\"width: 35.0%\">\n<col style=\"width: 25.2%\">") {
		t.Fatalf("the sized table's shares were not widened to the headings:\n%s", got)
	}
}

// TestTablePartsByMeasure locks the column sets a measure splits a table
// into: a part takes as many columns as head unbroken on the line, a heading
// wider than the line still joins the first column, and a sized table's
// shares widen to what its headings need.
func TestTablePartsByMeasure(t *testing.T) {
	labelled := func(labels ...string) []queryexec.Column {
		columns := make([]queryexec.Column, len(labels))
		for i, label := range labels {
			columns[i] = queryexec.Column{}.WithLabel(label)
		}
		return columns
	}
	// Four columns have half the dense line: the headings need 7, 8, 13 and 4
	// characters, the first at least 18 % of the line, so they overrun a line
	// of 60's half but fit the line whole, and a dense line of 30 in two parts.
	four := labelled("name", "alpha beta", "gammadelta", "x")
	if got := tableParts(four, 0, 60); !got.wide || !reflect.DeepEqual(got.parts, [][]int{{0, 1, 2, 3}}) {
		t.Fatalf("layout on 60 = %+v, want every column on the dense line", got)
	}
	if got := tableParts(four, 0, 30); !got.wide || !reflect.DeepEqual(got.parts, [][]int{{0, 1, 2}, {0, 3}}) {
		t.Fatalf("layout on 30 = %+v", got)
	}
	if got := tableParts(labelled("name", "a", "b"), 0, 60); got.wide || len(got.parts) != 1 {
		t.Fatalf("layout of a fitting table = %+v, want its own line", got)
	}
	if line := tableLine(140, 7); line != 112 {
		t.Fatalf("tableLine(140, 7) = %d", line)
	}
	// TableColumns still caps a part the measure would let grow.
	if got := tableParts(labelled("name", "a", "b", "c"), 3, 80).parts; !reflect.DeepEqual(got, [][]int{{0, 1, 2}, {0, 3}}) {
		t.Fatalf("capped parts = %v", got)
	}
	// A heading wider than the line sets beside the first column alone.
	if got := tableParts(labelled("name", "a", strings.Repeat("w", 50), "b"), 0, 40).parts; !reflect.DeepEqual(got, [][]int{{0, 1}, {0, 2}, {0, 3}}) {
		t.Fatalf("overwide parts = %v", got)
	}
	if need := headingNeed(queryexec.Column{}.WithLabel("Time Limit (s)")); need != 8 {
		t.Fatalf("headingNeed = %d, want the longest word and the padding", need)
	}
	sized := []queryexec.Column{
		queryexec.Column{}.WithLabel("name").WithWidth(774),
		queryexec.Column{}.WithLabel("postSegXchgTimeLimit").WithWidth(105),
	}
	// The second heading needs 23 of 100 characters, over its 105/879 share,
	// so it is pinned there and the first column takes the rest.
	shares := columnShares(sized, 100)
	if len(shares) != 2 || math.Abs(shares[0]-0.77) > 0.001 || math.Abs(shares[1]-0.23) > 0.001 {
		t.Fatalf("widened shares = %v", shares)
	}
	if shares := columnShares(sized, 0); math.Abs(shares[1]-105.0/879) > 0.001 {
		t.Fatalf("unmeasured shares = %v", shares)
	}
	// Ten value headings of 7 characters need 100 of 120; the first column
	// keeps its least share of 18 % rather than the 20 characters left, so a
	// part holds the nine that fit beside it, and the eleven together share
	// the line in proportion to their needs.
	dense := []queryexec.Column{queryexec.Column{}.WithLabel("name").WithWidth(774)}
	for i := 0; i < 10; i++ {
		dense = append(dense, queryexec.Column{}.WithLabel("abcdefg").WithWidth(105))
	}
	if got := tableParts(dense, 0, 120).parts; !reflect.DeepEqual(got, [][]int{{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, {0, 10}}) {
		t.Fatalf("dense parts = %v", got)
	}
	if shares := columnShares(dense, 120); math.Abs(shares[0]-firstColumnMinShare/(firstColumnMinShare+10.0/12)) > 0.001 || math.Abs(sum(shares)-1) > 1e-9 {
		t.Fatalf("dense shares = %v, want the needs scaled to the line", shares)
	}
	// An unsized wide table shares its width evenly, held to the headings:
	// eight columns take an eighth each on a line of 112, but a heading of 18
	// characters needs 21 of them, and the first column its 18 %.
	even := evenShares(labelled("name", "a", "b", "c", "d", "e", "f", "integrationTimePIT"), 112)
	if math.Abs(even[7]-21.0/112) > 0.001 || math.Abs(even[0]-firstColumnMinShare) > 0.001 || math.Abs(even[1]-even[2]) > 0.001 {
		t.Fatalf("even shares = %v", even)
	}
	if columnShares(labelled("name", "a"), 112) != nil {
		t.Fatal("an unsized table took shares")
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
		if got := tableParts(make([]queryexec.Column, tc.columns), tc.limit, 0).parts; !reflect.DeepEqual(got, tc.want) {
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

// TestTableColumnsOfOneRefused locks that a limit no continuation table keeps
// — one, or a negative number — is refused by both backends, and that 0
// leaves every table whole.
func TestTableColumnsOfOneRefused(t *testing.T) {
	document := fixtureDocument(t, sizedReportPath(), sizedReport)
	for _, columns := range []int{1, -1} {
		_, err := Markdown(document, MarkdownOptions{TableColumns: columns})
		var typed *Error
		if !errors.As(err, &typed) || typed.Kind != ErrorTableColumns || typed.Count != columns || !strings.Contains(err.Error(), "at least 2") {
			t.Errorf("Markdown with %d columns: error = %v", columns, err)
		}
		_, err = HTML(document, HTMLOptions{TableColumns: columns})
		if !errors.As(err, &typed) || typed.Kind != ErrorTableColumns || typed.Count != columns || typed.Form != "HTML" {
			t.Errorf("HTML with %d columns: error = %v", columns, err)
		}
	}
	if parts := tableParts(make([]queryexec.Column, 20), 0, 0).parts; len(parts) != 1 || len(parts[0]) != 20 {
		t.Errorf("tableParts(20, 0) = %v, want one part of every column", parts)
	}
}
