package examples

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan/markup"
)

// The AsciiDoc note lives in a directory of its own so that the commands over
// examples/self-model/*.sysml — make self-model, -render-documents, the
// architecture document's tests — never need the converter.
const (
	asciiDocNoteDir      = "self-model/asciidoc"
	asciiDocNoteFile     = "markup.sysml"
	asciiDocNoteDocument = "OpenSysMLAsciiDoc::MarkupNote"
	pdfToolchainRequired = "OPENSYSML_REQUIRE_PDF_TOOLCHAIN"
)

func readAsciiDocNote(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(asciiDocNoteDir, asciiDocNoteFile))
	if err != nil {
		t.Fatalf("read the AsciiDoc note: %v", err)
	}
	return string(content)
}

// requireAsciiDocConverter skips the test when the converter is absent, unless
// the environment claims the PDF toolchain is installed, when absence is a failure.
func requireAsciiDocConverter(t *testing.T) {
	t.Helper()
	_, err := markup.AsciiDoc{}.Parse("A probe.\n")
	var tool *markup.ToolError
	if errors.As(err, &tool) && tool.Missing {
		if required := os.Getenv(pdfToolchainRequired); required != "" {
			t.Fatalf("%s=%s but %v", pdfToolchainRequired, required, err)
		}
		t.Skipf("%s is unset and %s is not on PATH: run ./scripts/download-doc-pdf-toolchain.sh and point %s at build/doc-pdf/asciidoctor/asciidoctor-json.js",
			markup.AsciiDocEnv, tool.Tool, markup.AsciiDocEnv)
	}
	if err != nil {
		t.Fatalf("the AsciiDoc converter does not parse a one-line body: %v", err)
	}
}

// TestSelfModelAsciiDocNoteSource pins the note's shape without the converter:
// its bodies, the runs they carry, and that no file the architecture document
// is rendered from carries an AsciiDoc body.
func TestSelfModelAsciiDocNoteSource(t *testing.T) {
	note := readAsciiDocNote(t)
	if n, want := strings.Count(note, `rep language "asciidoc"`), 3; n != want {
		t.Errorf("%d AsciiDoc bodies in the note, want %d", n, want)
	}
	if n := strings.Count(note, `rep language "markdown"`); n != 0 {
		t.Errorf("%d Markdown bodies in the note, want none", n)
	}
	if n, want := len(regexp.MustCompile(`sysml:value\[`).FindAllString(note, -1)), 9; n != want {
		t.Errorf("%d value runs in the note, want %d", n, want)
	}
	if n, want := strings.Count(note, "sysml:ref["), 1; n != want {
		t.Errorf("%d sysml:ref runs in the note, want %d", n, want)
	}
	for _, want := range []string{"<<parsers>>", "<<converter>>", "<<languages>>", "[[converter]]\n", "== The converter"} {
		if !strings.Contains(note, want) {
			t.Errorf("the note lost %q", want)
		}
	}
	if n, want := strings.Count(note, "part def MarkupNote :> Document"), 1; n != want {
		t.Errorf("%d MarkupNote definitions, want %d", n, want)
	}

	for _, name := range selfModelFiles(t) {
		if strings.Contains(readModelFile(t, name), `rep language "asciidoc"`) {
			t.Errorf("%s carries an AsciiDoc body: the files under %s must render without the converter", name, selfModelDir)
		}
	}
}

// TestSelfModelAsciiDocNoteRendersInstalled renders the note through the
// pinned converter in every form and checks that each AsciiDoc idiom — value
// runs, cross-references, a heading that opens a section, lists, emphasis, an
// admonition — reached the output, with the values read from the model.
func TestSelfModelAsciiDocNoteRendersInstalled(t *testing.T) {
	requireAsciiDocConverter(t)

	var files []repl.SourceFile
	for _, name := range selfModelFiles(t) {
		files = append(files, repl.SourceFile{Name: name, Text: readModelFile(t, name)})
	}
	files = append(files, repl.SourceFile{Name: asciiDocNoteFile, Text: readAsciiDocNote(t)})
	session := repl.NewSession()
	session.SubmitFiles(files)
	if session.HasErrors() {
		t.Fatalf("the self-model with its AsciiDoc note does not analyse:\n%s", strings.Join(session.DiagnosticLines(), "\n"))
	}

	markdown, err := session.RenderDocumentMarkdown(asciiDocNoteDocument, docrender.MarkdownOptions{})
	if err != nil {
		t.Fatalf("render the note as Markdown: %v", err)
	}
	html, err := session.RenderDocumentHTML(asciiDocNoteDocument, docrender.HTMLOptions{})
	if err != nil {
		t.Fatalf("render the note as HTML: %v", err)
	}
	asciidoc, err := session.RenderDocumentAsciiDoc(asciiDocNoteDocument, docrender.AsciiDocOptions{})
	if err != nil {
		t.Fatalf("render the note as AsciiDoc: %v", err)
	}
	for _, literal := range []string{"sysml:value[", "sysml:ref[", "<<parsers>>", "<<converter>>", "<<languages>>"} {
		for form, out := range map[string]string{"Markdown": markdown, "HTML": html, "AsciiDoc": asciidoc} {
			if strings.Contains(out, literal) {
				t.Errorf("the %s rendering still contains %q: a run was not expanded", form, literal)
			}
		}
	}

	surfaces := readModelFile(t, "surfaces.sysml")
	markupLanguages, _ := declaredString(surfaces, "markupLanguages")
	contentKinds, _ := declaredString(surfaces, "contentKinds")
	runKinds, _ := declaredString(surfaces, "runKinds")
	toolEnv, _ := declaredString(surfaces, "toolEnv")
	converterScript, _ := declaredString(surfaces, "converterScript")
	parsers := regexp.MustCompile(`part def MarkupParser[^}]*goPackage = "([^"]+)"`).FindStringSubmatch(surfaces)
	if parsers == nil {
		t.Fatal("surfaces.sysml no longer declares the markup parsers' Go package")
	}
	escape := func(s string) string { return strings.ReplaceAll(s, "_", `\_`) }
	for _, want := range []string{
		"# Prose in Two Markups",
		"may be written in\u2009—\u2009" + markupLanguages + "\u2009—\u2009and",
		"every document is made of: " + contentKinds + ".",
		"the runs are " + runKinds + ",",
		"is immutable (immutablePlans: true)",
		"[One planner, two parsers](#parsers)",
		"[The converter](#converter)",
		"[The two parsers, and which one leaves the process](#parsers-languages) is a query over the planner's siblings",
		"## One planner, two parsers",
		"| asciidocProse | asciidoc | true |",
		"| markdownProse | markdown | false |",
		"## The converter",
		"(needsExternalTool: true)",
		"run through " + converterScript + " and found by the variable " + escape(toolEnv) + " or as `asciidoctor-json` on the path",
		"- Both parsers live in " + parsers[1] + ".",
		"*AsciiDoc*",
		"**never**",
		"**Note:** The Markdown parser (markdown) runs in the process",
	} {
		if !strings.Contains(markdown, want) {
			t.Errorf("the Markdown rendering does not contain %q", want)
		}
	}
	for _, want := range []string{
		"<h2>One planner, two parsers</h2>", "<h2>The converter</h2>",
		`href="#parsers"`, `href="#converter"`, `href="#parsers-languages"`, `id="converter"`,
		`data-query="OpenSysMLDocument::MarkupLanguages"`,
		"<em>AsciiDoc</em>", "<strong>never</strong>",
		`<li class="sysml-item">`, "found by the variable OPENSYSML_ASCIIDOCTOR",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("the HTML rendering does not contain %q", want)
		}
	}
	for _, want := range []string{
		"= Prose in Two Markups",
		"<<parsers,One planner, two parsers>>", "<<converter,The converter>>",
		"[[parsers]]\n== One planner, two parsers",
		"[[converter]]\n== The converter",
		"| asciidocProse | asciidoc | true",
		"_AsciiDoc_", "*never*",
		"* `./scripts/download-doc-pdf-toolchain.sh` installs the pinned converter.",
		"found by the variable OPENSYSML&#95;ASCIIDOCTOR",
	} {
		if !strings.Contains(asciidoc, want) {
			t.Errorf("the AsciiDoc rendering does not contain %q", want)
		}
	}
}
