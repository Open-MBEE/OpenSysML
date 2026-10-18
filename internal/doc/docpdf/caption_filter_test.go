package docpdf

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docir"
	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

// tableDiagramDocument is a report whose captioned artwork is, in order, two
// table-kind diagrams, a Mermaid diagram, a matrix diagram, a table whose
// caption is padded to CommonMark's indented-code width, a table whose caption
// is blank and a captioned table, with emphasized prose between them that
// repeats a caption's text.
func tableDiagramDocument(t *testing.T) *docir.Document {
	t.Helper()
	return sourceDocument(t, "tables.sysml", `package Tables {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	private import StandardViewDefinitions::*;

	part def Subsystem { attribute mass : Real; }
	part imagingChain {
		part optics : Subsystem { attribute redefines mass = 8.5; }
		part sensor : Subsystem { attribute redefines mass = 2.0; }
		connect optics to sensor;
	}
	part emptyChain;
	view def RelationshipMatrix :> GridView {
		filter @SysML::ConnectionUsage;
	}
	view imagingMatrix : RelationshipMatrix {
		expose imagingChain::**;
	}

	calc def Names :> Query {
		in root : Element;
		Project(source = WhereType(source = OwnedElements(source = root), type = "Tables::Subsystem"), properties = ("name"))
	}

	part def Report :> Document {
		attribute redefines title = "Tables Report";
		part masses : Diagram {
			attribute redefines caption = "Masses";
			attribute redefines kind = "table";
			ref redefines source = imagingChain;
		}
		part rest : Diagram {
			attribute redefines caption = "Everything else";
			attribute redefines kind = "table";
			ref redefines source = emptyChain;
		}
		part aside : Paragraph {
			part a : Span { attribute redefines text = "Masses"; attribute redefines style = "emphasis"; }
		}
		part chain : Diagram {
			attribute redefines caption = "Imaging chain";
			attribute redefines kind = "interconnection";
			ref redefines source = imagingChain;
		}
		part relationships : Diagram {
			attribute redefines caption = "Relationships";
			ref redefines source = imagingMatrix;
		}
		part names : Table {
			attribute redefines caption = "    Subsystems by name ";
			calc rows : Names { in root = imagingChain; }
		}
		part unnamed : Table {
			attribute redefines caption = "   ";
			calc rows : Names { in root = imagingChain; }
		}
		part total : Table {
			attribute redefines caption = "Total mass";
			calc rows : Names { in root = imagingChain; }
		}
	}
}
`, "Tables::Report")
}

// TestArtworkFilterMarksCaptionsPastTableRenderings runs the caption filter
// under an installed pandoc over a document whose table and matrix diagrams put a
// rendering comment between caption and table, and checks every caption is
// marked while the emphasized prose between them is not.
func TestArtworkFilterMarksCaptionsPastTableRenderings(t *testing.T) {
	pandoc, err := pandocTool.locate("")
	if err != nil {
		t.Skipf("pandoc not installed: %v", err)
	}
	document := tableDiagramDocument(t)
	captions := docrender.Captions(document, false)
	if want := []string{"Masses", "Everything else", "Imaging chain", "Relationships", "Subsystems by name", "Total mass"}; strings.Join(captions, "|") != strings.Join(want, "|") {
		t.Fatalf("captions = %q, want %q", captions, want)
	}
	markdown, err := docrender.Markdown(document, docrender.MarkdownOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"*Masses*\n\n<!-- table rendering",
		"*Everything else*\n\n<!-- table rendering",
		"*Relationships*\n\n<!-- Tables::imagingMatrix — matrix rendering",
		"| Source / Target |",
		"\n*Subsystems by name*\n",
		"*Total mass*\n\n|",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("Markdown lacks %q:\n%s", want, markdown)
		}
	}
	for _, literal := range []string{"    *Subsystems", "*   *"} {
		if strings.Contains(markdown, literal) {
			t.Fatalf("Markdown carries a caption's padding %q:\n%s", literal, markdown)
		}
	}
	html := filteredHTML(t, pandoc, markdown, captions)
	if !strings.Contains(html, "<p><em>Masses</em></p>") {
		t.Errorf("emphasized prose between the captions not kept as prose:\n%s", html)
	}
}

// TestArtworkFilterMarksCaptionOfEmptyTableRendering checks a caption is
// marked when its table-kind diagram rendered nothing: the rendering comment
// is followed by the reason rather than a table.
func TestArtworkFilterMarksCaptionOfEmptyTableRendering(t *testing.T) {
	pandoc, err := pandocTool.locate("")
	if err != nil {
		t.Skipf("pandoc not installed: %v", err)
	}
	rendering := &view.Rendering{Kind: view.KindTable, Notices: []string{"attribute mass is not projected"}}
	if !rendering.Empty() {
		t.Fatal("rendering is not empty")
	}
	markdown := "# Report\n\n*Nothing to tabulate*\n\n" + rendering.Markdown() + "\n*Trailing*\n\n| a |\n| --- |\n| 1 |\n"
	filteredHTML(t, pandoc, markdown, []string{"Nothing to tabulate", "Trailing"})
}

// filteredHTML converts markdown to HTML through pandoc with the caption
// filter written for captions, and checks each caption is marked once and
// nothing else is; the HTML comes back with its whitespace runs collapsed,
// since pandoc wraps paragraphs.
func filteredHTML(t *testing.T, pandoc, markdown string, captions []string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, markdownFileName), []byte(markdown), 0o600); err != nil {
		t.Fatal(err)
	}
	filter, err := writeArtworkFilter(dir, nil, formulas{}, captions)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(pandoc, "--from", "markdown", "--to", "html", "--lua-filter", filter, markdownFileName) // #nosec G204 -- pandoc from the toolchain, fixed arguments
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pandoc: %v\n%s", err, out)
	}
	html := strings.Join(strings.Fields(string(out)), " ")
	for _, caption := range captions {
		if !strings.Contains(html, `<span class="caption"><em>`+caption+`</em></span>`) {
			t.Errorf("caption %q not marked:\n%s", caption, html)
		}
	}
	if got := strings.Count(html, `<span class="caption">`); got != len(captions) {
		t.Errorf("captions marked = %d, want %d:\n%s", got, len(captions), html)
	}
	return html
}

func TestArtworkFilterMarksTablesByHeaderWidth(t *testing.T) {
	pandoc, err := pandocTool.locate("")
	if err != nil {
		t.Skipf("pandoc not installed: %v", err)
	}
	pipeTable := func(columns int) string {
		headers := make([]string, columns)
		rule := make([]string, columns)
		values := make([]string, columns)
		for i := range headers {
			headers[i], rule[i], values[i] = "column", "---", "value"
		}
		return "| " + strings.Join(headers, " | ") + " |\n| " + strings.Join(rule, " | ") +
			" |\n| " + strings.Join(values, " | ") + " |"
	}
	markdown := strings.Join([]string{pipeTable(6), pipeTable(7), pipeTable(11)}, "\n\n")
	html := filteredHTML(t, pandoc, markdown, nil)
	for _, want := range []string{
		`<table>`,
		`<table class="sysml-table-landscape">`,
		`<table class="sysml-table-landscape sysml-table-dense">`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("filtered HTML lacks %q:\n%s", want, html)
		}
	}

	dir := t.TempDir()
	filter, err := writeArtworkFilter(dir, nil, formulas{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(filepath.Join(dir, filter))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"local landscapeColumns = " + strconv.Itoa(docrender.WideTableColumns),
		"local denseColumns = " + strconv.Itoa(docrender.DenseTableColumns),
	} {
		if !strings.Contains(string(script), want) {
			t.Errorf("filter lacks %q:\n%s", want, script)
		}
	}
}

// TestArtworkFilterMarksCaptionPastFallbackNotice checks a positioned
// diagram's caption is marked when, without Graphviz, the Markdown backend
// puts the fallback notice between the caption and the Mermaid fence — and
// that the notice itself stays plain emphasis.
func TestArtworkFilterMarksCaptionPastFallbackNotice(t *testing.T) {
	pandoc, err := pandocTool.locate("")
	if err != nil {
		t.Skipf("pandoc not installed: %v", err)
	}
	document := fixtureDocument(t, filepath.Join("testdata", "cameo_report.sysml"), "Instrument::CameoReport")
	captions := docrender.Captions(document, false)
	markdown, err := docrender.Markdown(document, docrender.MarkdownOptions{WithoutGraphviz: true})
	if err != nil {
		t.Fatal(err)
	}
	if want := "*PEAS states, as Cameo drew them*\n\n*" + docrender.GraphvizFallbackNotice + "*\n\n```mermaid"; !strings.Contains(markdown, want) {
		t.Fatalf("Markdown lacks %q:\n%s", want, markdown)
	}
	html := filteredHTML(t, pandoc, markdown, captions)
	if !strings.Contains(html, "<p><em>"+docrender.GraphvizFallbackNotice+"</em></p>") {
		t.Errorf("fallback notice not kept as emphasized prose:\n%s", html)
	}
}
