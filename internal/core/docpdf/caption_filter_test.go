package docpdf

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/docir"
	"github.com/Open-MBEE/OpenSysML/internal/core/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// tableDiagramDocument is a report whose captioned artwork is, in order, two
// table-kind diagrams, a Mermaid diagram, a table whose caption is padded to
// CommonMark's indented-code width, a table whose caption is blank and a
// captioned table, with an emphasized paragraph of prose between them that
// repeats a caption's text.
func tableDiagramDocument(t *testing.T) *docir.Document {
	t.Helper()
	return sourceDocument(t, "tables.sysml", `package Tables {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	part def Subsystem { attribute mass : Real; }
	part imagingChain {
		part optics : Subsystem { attribute redefines mass = 8.5; }
		part sensor : Subsystem { attribute redefines mass = 2.0; }
		connect optics to sensor;
	}
	part emptyChain;

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
// under an installed pandoc over a document whose table-kind diagrams put a
// rendering comment between caption and table, and checks every caption is
// marked while the emphasized prose between them is not.
func TestArtworkFilterMarksCaptionsPastTableRenderings(t *testing.T) {
	pandoc, err := pandocTool.locate("")
	if err != nil {
		t.Skipf("pandoc not installed: %v", err)
	}
	document := tableDiagramDocument(t)
	captions := docrender.Captions(document)
	if want := []string{"Masses", "Everything else", "Imaging chain", "Subsystems by name", "Total mass"}; strings.Join(captions, "|") != strings.Join(want, "|") {
		t.Fatalf("captions = %q, want %q", captions, want)
	}
	markdown, err := docrender.Markdown(document, docrender.MarkdownOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"*Masses*\n\n<!-- table rendering", "*Everything else*\n\n<!-- table rendering", "\n*Subsystems by name*\n", "*Total mass*\n\n|"} {
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
// nothing else is.
func filteredHTML(t *testing.T, pandoc, markdown string, captions []string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, markdownFileName), []byte(markdown), 0o600); err != nil {
		t.Fatal(err)
	}
	filter, err := writeArtworkFilter(dir, view.FormMermaid, nil, formulas{}, captions)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(pandoc, "--from", "markdown", "--to", "html", "--lua-filter", filter, markdownFileName) // #nosec G204 -- pandoc from the toolchain, fixed arguments
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pandoc: %v\n%s", err, out)
	}
	html := string(out)
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
