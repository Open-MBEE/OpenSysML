package markup

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan/markup/markuptest"
)

const (
	asciiDocBodyFixture = "testdata/asciidoc_body.adoc"
	asciiDocJSONFixture = "testdata/asciidoc_body.json"
)

// toolchainRequiredEnv is set in CI after scripts/download-doc-pdf-toolchain.sh
// has run, so an absent converter fails the installed-tool test instead of
// skipping it; the converter is provisioned by that script beside the PDF tools.
const toolchainRequiredEnv = "OPENSYSML_REQUIRE_PDF_TOOLCHAIN"

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// decodeFixture maps the recorded converter answer onto blocks of the body,
// so the mapping is tested without node.
func decodeFixture(t *testing.T) (string, []Block) {
	t.Helper()
	body := readFixture(t, asciiDocBodyFixture)
	blocks, err := decodeAsciiDoc(body, []byte(readFixture(t, asciiDocJSONFixture)))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body, blocks
}

func runSummary(runs []Run) []string {
	out := make([]string, len(runs))
	for i, run := range runs {
		switch run.Kind {
		case RunText:
			out[i] = string(run.Style) + ":" + run.Text
		case RunLink, RunRef, RunValue:
			out[i] = string(run.Kind) + ":" + run.Destination + ":" + run.Text
		default:
			out[i] = string(run.Kind) + ":" + run.Text
		}
	}
	return out
}

// Every supported construct maps onto the block and run the manual documents.
func TestAsciiDocFixtureBlocks(t *testing.T) {
	body, blocks := decodeFixture(t)
	kinds := make([]BlockKind, len(blocks))
	for i, block := range blocks {
		kinds[i] = block.Kind
	}
	want := []BlockKind{BlockParagraph, BlockList, BlockParagraph, BlockList, BlockTable, BlockFormula, BlockImage}
	if !reflect.DeepEqual(kinds, want) {
		t.Fatalf("block kinds = %v, want %v", kinds, want)
	}

	paragraph := runSummary(blocks[0].Runs)
	wantRuns := []string{
		"plain:Plain ", "emphasis:em", "plain: ", "strong:strong", "plain: ", "code:co\\de", "plain: ",
		"link:https://x.io/a:lbl", "plain: ", "link:https://y.io:https://y.io", "plain:\n",
		"ref:Optics::mirror:", "plain: ", "ref:Optics::mount:the mount", "plain: ", "ref:summary:the summary", "plain: ", "ref:notes:",
		"plain: ", "math:m \\propto D^2", "plain: \u00a9 end.",
	}
	if !reflect.DeepEqual(paragraph, wantRuns) {
		t.Errorf("paragraph runs = %q\nwant %q", paragraph, wantRuns)
	}
	if em := blocks[0].Runs[1]; body[em.Span.Start:em.Span.Stop] != "em" {
		t.Errorf("emphasis run spans %q", body[em.Span.Start:em.Span.Stop])
	}
	if ref := blocks[0].Runs[13]; body[ref.Span.Start:ref.Span.Stop] != "Optics::mount" || lineOf(body, ref.Span.Start) != 2 {
		t.Errorf("ref run spans %q on line %d", body[ref.Span.Start:ref.Span.Stop], lineOf(body, ref.Span.Start))
	}

	if list := blocks[1]; list.Ordered || len(list.Items) != 2 ||
		!reflect.DeepEqual(runSummary(list.Items[1].Runs), []string{"plain:two ", "emphasis:soft"}) ||
		lineOf(body, list.Items[1].Span.Start) != 5 {
		t.Errorf("bullet list = %+v", list)
	}
	if note := runSummary(blocks[2].Runs); !reflect.DeepEqual(note, []string{"strong:Note:", "plain: Mind the ", "strong:gap", "plain:."}) {
		t.Errorf("admonition runs = %q", note)
	}
	if list := blocks[3]; !list.Ordered || len(list.Items) != 2 || list.Items[0].Runs[0].Text != "first" {
		t.Errorf("numbered list = %+v", list)
	}

	table := blocks[4]
	if table.Caption != "Masses" || !reflect.DeepEqual(table.ColumnLabels, []string{"Part", "Mass"}) ||
		!reflect.DeepEqual(table.ColumnWidths, []int{3, 1}) {
		t.Errorf("table header = %+v", table)
	}
	if len(table.Rows) != 2 || !reflect.DeepEqual(table.Rows[0].Cells, []string{"Mirror", "12 kg"}) ||
		!reflect.DeepEqual(table.Rows[1].Cells, []string{"Mount", "4 kg"}) {
		t.Errorf("table rows = %+v", table.Rows)
	}
	if !strings.HasPrefix(body[table.Span.Start:], ".Masses\n[cols") || !strings.HasSuffix(body[:table.Span.Stop], "|===") {
		t.Errorf("table spans %q", body[table.Span.Start:table.Span.Stop])
	}
	if formula := blocks[5]; formula.Source != `m \propto D^2` || formula.Caption != "Scaling" {
		t.Errorf("formula = %+v", formula)
	}
	if image := blocks[6]; image.Location != "https://example.test/figures/optics.png" || image.Alt != "The optics" || image.Caption != "Optics" {
		t.Errorf("image = %+v", image)
	}
	for i := 1; i < len(blocks); i++ {
		if blocks[i-1].Span.Stop > blocks[i].Span.Start {
			t.Errorf("block %d ends at %d, after block %d starts at %d", i-1, blocks[i-1].Span.Stop, i, blocks[i].Span.Start)
		}
	}
}

// A `sysml:value` macro is a value run labelled by its second positional
// attribute, placed where it occurs in the body.
func TestAsciiDocValueRuns(t *testing.T) {
	body := "see sysml:value[a.mass] and sysml:value[a.b, Mass:] end\n"
	answer := `{"blocks":[{"kind":"paragraph","line":1,"runs":[{"kind":"text","style":"plain","text":"see "},{"kind":"value","target":"a.mass","text":""},{"kind":"text","style":"plain","text":" and "},{"kind":"value","target":"a.b","text":"Mass:"},{"kind":"text","style":"plain","text":" end"}]}]}`
	blocks, err := decodeAsciiDoc(body, []byte(answer))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("got %d blocks, want one paragraph", len(blocks))
	}
	want := []string{"plain:see ", "value:a.mass:", "plain: and ", "value:a.b:Mass:", "plain: end"}
	if got := runSummary(blocks[0].Runs); !reflect.DeepEqual(got, want) {
		t.Errorf("runs = %q, want %q", got, want)
	}
	for i, at := range []string{"a.mass", "a.b"} {
		run := blocks[0].Runs[1+2*i]
		if got := body[run.Span.Start:run.Span.Stop]; got != at {
			t.Errorf("value run %d spans %q, want %q", i, got, at)
		}
	}
}

// Each construct no content block carries is refused by name, with the body
// line it starts on.
func TestAsciiDocRefusals(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		json      string
		construct string
		line      int
	}{
		{"document header", "= Title\n\npara\n", `{"blocks":[{"kind":"header","line":1},{"kind":"paragraph","line":3,"runs":[]}]}`, "document header", 1},
		{"section", "para\n\n== Heading\n", `{"blocks":[{"kind":"paragraph","line":1,"runs":[]},{"kind":"section","line":3,"title":"Heading"}]}`, "section heading", 3},
		{"listing", "----\ncode\n----\n", `{"blocks":[{"kind":"listing","line":1}]}`, "listing block", 1},
		{"nested list", "* a\n** b\n", `{"blocks":[{"kind":"list","line":1,"items":[{"line":1,"runs":[],"blocks":[{"kind":"list","line":2,"items":[]}]}]}]}`, "nested list", 2},
		{"attached paragraph", "* a\n+\npara\n", `{"blocks":[{"kind":"list","line":1,"items":[{"line":1,"runs":[],"blocks":[{"kind":"paragraph","line":3,"runs":[]}]}]}]}`, "paragraph attached to a list item", 3},
		{"headerless table", "|===\n|a\n|===\n", `{"blocks":[{"kind":"table","line":1,"cols":[],"header":[],"rows":[[{"runs":[]}]],"footer":[]}]}`, "table without a header row", 1},
		{"table footer", "|===\n|h\n\n|a\n|f\n|===\n", `{"blocks":[{"kind":"table","line":1,"cols":[],"header":[[{"runs":[]}]],"rows":[],"footer":[[{"runs":[]}]]}]}`, "table footer", 1},
		{"styled cell", "|===\n|h\n\n|*a*\n|===\n", `{"blocks":[{"kind":"table","line":1,"cols":[],"header":[[{"runs":[]}]],"rows":[[{"runs":[{"kind":"text","style":"strong","text":"a"}]}]],"footer":[]}]}`, "strong text in a table cell", 1},
		{"ref in cell", "|===\n|h\n\n|sysml:ref[x]\n|===\n", `{"blocks":[{"kind":"table","line":1,"cols":[],"header":[[{"runs":[]}]],"rows":[[{"runs":[{"kind":"ref","target":"x","text":""}]}]],"footer":[]}]}`, "sysml:ref macro in a table cell", 1},
		{"value in cell", "|===\n|h\n\n|sysml:value[x]\n|===\n", `{"blocks":[{"kind":"table","line":1,"cols":[],"header":[[{"runs":[]}]],"rows":[[{"runs":[{"kind":"value","target":"x","text":""}]}]],"footer":[]}]}`, "sysml:value macro in a table cell", 1},
		{"value without target", "sysml:value[]\n", `{"blocks":[{"kind":"paragraph","line":1,"runs":[{"kind":"value","target":"","text":""}]}]}`, "sysml:value macro without a target", 1},
		{"inline image", "an image:x.png[] here\n", `{"blocks":[{"kind":"paragraph","line":1,"runs":[{"kind":"image","target":"x.png"}]}]}`, "inline image", 1},
		{"footnote", "a\n\nb footnote:[n]\n", `{"blocks":[{"kind":"paragraph","line":1,"runs":[]},{"kind":"paragraph","line":3,"runs":[{"kind":"unsupported","construct":"footnote"}]}]}`, "footnote", 3},
		{"nested quoted text", "*a _b_*\n", `{"blocks":[{"kind":"paragraph","line":1,"runs":[{"kind":"unsupported","construct":"nested quoted text"}]}]}`, "nested quoted text", 1},
		{"asciimath", "[asciimath]\n++++\nx\n++++\n", `{"blocks":[{"kind":"stem","line":2,"style":"asciimath","source":"x"}]}`, "asciimath stem block", 1},
		{"titled paragraph", ".T\npara\n", `{"blocks":[{"kind":"paragraph","line":2,"title":"T","runs":[]}]}`, "title on a paragraph", 1},
		{"styled title", ".*T*\n|===\n|h\n\n|a\n|===\n", `{"blocks":[{"kind":"table","line":2,"title":"","styledTitle":true,"cols":[],"header":[[{"runs":[]}]],"rows":[],"footer":[]}]}`, "styled block title", 1},
		{"xref without target", "<<>>\n", `{"blocks":[{"kind":"paragraph","line":1,"runs":[{"kind":"xref","target":"","text":""}]}]}`, "cross reference without a target", 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := decodeAsciiDoc(test.body, []byte(test.json))
			var unsupported *UnsupportedConstructError
			if !errors.As(err, &unsupported) {
				t.Fatalf("error = %v, want an unsupported construct", err)
			}
			if unsupported.Construct != test.construct || unsupported.Line != test.line {
				t.Errorf("refused %q at line %d, want %q at line %d", unsupported.Construct, unsupported.Line, test.construct, test.line)
			}
		})
	}
}

// An answer that is not the converter's tree is the tool failing, not a body error.
func TestAsciiDocUnreadableAnswer(t *testing.T) {
	_, err := decodeAsciiDoc("a\n", []byte("not json"))
	var tool *ToolError
	if !errors.As(err, &tool) || tool.Missing || !strings.Contains(tool.Error(), "unreadable answer") {
		t.Fatalf("error = %v, want the converter failing with an unreadable answer", err)
	}
}

// Without the converter, parsing fails naming the provisioning script and the
// variable; the language is still served, so the planner reports the tool, not
// an unsupported language.
func TestAsciiDocMissingConverter(t *testing.T) {
	t.Setenv(AsciiDocEnv, filepath.Join(t.TempDir(), "no-such-converter"))
	_, err := AsciiDoc{}.Parse("a\n")
	var tool *ToolError
	if !errors.As(err, &tool) || !tool.Missing {
		t.Fatalf("error = %v, want the converter missing", err)
	}
	msg := tool.Error()
	for _, want := range []string{"download-doc-pdf-toolchain.sh", AsciiDocEnv, "no-such-converter"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q lacks %q", msg, want)
		}
	}
}

// fakeConverter writes an executable that answers stdout and stderr and exits
// with code, standing in for the node converter.
func fakeConverter(t *testing.T, stdout, stderr string, code int) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake converter is a shell script")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh to run the fake converter")
	}
	path := filepath.Join(t.TempDir(), "asciidoctor-json")
	script := "#!/bin/sh\ncat >/dev/null\n"
	if stderr != "" {
		script += "printf '%s\\n' " + shellQuote(stderr) + " >&2\n"
	}
	if stdout != "" {
		script += "printf '%s' " + shellQuote(stdout) + "\n"
	}
	script += "exit " + string(rune('0'+code)) + "\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil { // #nosec G306 -- an executable test double
		t.Fatal(err)
	}
	return path
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// A converter that exits non-zero fails the parse with its message.
func TestAsciiDocConverterFailure(t *testing.T) {
	t.Setenv(AsciiDocEnv, fakeConverter(t, "", "asciidoctor: FAILED: unterminated block", 1))
	_, err := AsciiDoc{}.Parse("a\n")
	var tool *ToolError
	if !errors.As(err, &tool) || tool.Missing || !strings.Contains(tool.Error(), "unterminated block") {
		t.Fatalf("error = %v, want the converter failing with its message", err)
	}
}

// Through the registry, the converter's answer is mapped exactly as the recorded fixture is.
func TestAsciiDocParsesThroughTheConverter(t *testing.T) {
	converter, err := markuptest.RecordedConverter(t.TempDir(), markuptest.RecordingsDir)
	if err != nil {
		t.Skip(err)
	}
	t.Setenv(AsciiDocEnv, converter)
	body, want := decodeFixture(t)
	p, ok := For("ASCIIDOC")
	if !ok {
		t.Fatal("no AsciiDoc parser")
	}
	got, err := p.Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("blocks through the converter differ from the recorded fixture's:\n%+v\n%+v", got, want)
	}
}

// TestAsciiDocInstalledConverterMatchesTheFixture proves the recorded answer
// is what the pinned Asciidoctor.js converter writes today; it skips without
// the converter unless the toolchain is declared mandatory.
func TestAsciiDocInstalledConverterMatchesTheFixture(t *testing.T) {
	bodies, err := filepath.Glob(filepath.Join(markuptest.RecordingsDir, "*.adoc"))
	if err != nil || len(bodies) == 0 {
		t.Fatalf("no recorded bodies: %v", err)
	}
	for _, name := range bodies {
		output, err := runAsciiDoc(readFixture(t, name))
		var tool *ToolError
		if errors.As(err, &tool) && tool.Missing {
			if required := os.Getenv(toolchainRequiredEnv); required != "" {
				t.Fatalf("%s=%s but %v", toolchainRequiredEnv, required, err)
			}
			t.Skipf("AsciiDoc converter not installed: %v", err)
		}
		if err != nil {
			t.Fatal(err)
		}
		recorded := strings.TrimSuffix(name, ".adoc") + ".json"
		if want := readFixture(t, recorded); !bytes.Equal(bytes.TrimSpace(output), bytes.TrimSpace([]byte(want))) {
			t.Errorf("the installed converter answers\n%s\nwhere %s records\n%s", output, recorded, want)
		}
	}
	if _, err := (AsciiDoc{}).Parse("= Title\n\nbody\n"); err == nil || !strings.Contains(err.Error(), "document header") {
		t.Errorf("a document header parsed: %v", err)
	}
}
