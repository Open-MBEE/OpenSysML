package docpdf

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// dotMarkdown is a document whose one diagram is written as DOT: a `&` and a
// quoted ID check the source reaches the page escaped, not interpreted.
const dotMarkdown = "# T\n\n## Flow\n\n```dot\n// kind: action\ndigraph \"a & b\" {\n  \"n0\" -> \"n1\";\n}\n```\n\n<!-- caption -->\n*Figure 1\\. Flow*\n"

// plantumlMarkdown is a document whose one diagram is written as PlantUML: a
// creole label and a style block check the source reaches the page escaped.
const plantumlMarkdown = "# T\n\n## Flow\n\n```plantuml\n@startuml\n' action rendering\n<style>\nelement {\n  LineColor #181818\n}\n</style>\nstate \"**a & b**\" as n0 <<action>>\nn0 --> n1\n@enduml\n```\n\n<!-- caption -->\n*Figure 1\\. Flow*\n"

// withoutDiagramTools empties PATH and every diagram tool's override, so a
// test sees the tools absent whatever the machine has installed.
func withoutDiagramTools(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
	for _, env := range []string{MermaidEnv, DotEnv, JavaEnv, PlantUMLJarEnv} {
		t.Setenv(env, "")
	}
}

// fakeSVGTool writes a fake diagram tool that writes an SVG to the file its
// `-o`/`--output` argument names, or to stdout when it has neither, and logs
// its arguments and stdin to <name>.log in dir.
func fakeSVGTool(t *testing.T, dir, name, envVar string) string {
	t.Helper()
	log := filepath.Join(dir, name+".log")
	fakeTool(t, dir, name, envVar, `printf 'args:%s\n' "$*" >> "`+log+`"
out=""
while [ $# -gt 0 ]; do
  case "$1" in
    -o|--output) out="$2"; shift ;;
    -pipe) while IFS= read -r line; do printf '%s\n' "$line"; done >> "`+log+`" ;;
  esac
  shift
done
if [ -n "$out" ]; then
  printf '<svg xmlns="http://www.w3.org/2000/svg"><text>drawn by `+name+`</text></svg>' > "$out"
else
  printf '<svg xmlns="http://www.w3.org/2000/svg"><text>drawn by `+name+`</text></svg>'
fi
`)
	return log
}

// fakeWeasyPrint writes a fake converter that keeps the HTML it was given at
// seenPath and writes a PDF signature; shell builtins only, as PATH is empty.
func fakeWeasyPrint(t *testing.T, dir string) string {
	t.Helper()
	seenPath := filepath.Join(dir, "input-seen.html")
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv, `while IFS= read -r line; do printf '%s\n' "$line"; done < "$1" > "`+seenPath+`"
printf '%%PDF-1.7 fake' > "$2"
`)
	return seenPath
}

// A DOT fence parses to its own block; without Graphviz it is kept as source
// under a notice naming the variable to set, on both converter inputs, and
// no Mermaid CLI is looked for.
func TestDOTBlockWithoutGraphvizIsKeptAsSource(t *testing.T) {
	blocks, err := parseBlocks(dotMarkdown)
	if err != nil {
		t.Fatalf("parseBlocks: %v", err)
	}
	if len(blocks) != 4 || blocks[2].Kind != blockDOT || !strings.Contains(blocks[2].Source, `"n0" -> "n1";`) {
		t.Fatalf("blocks = %+v", blocks)
	}
	withoutDiagramTools(t)
	diagrams, err := renderDiagrams(t.TempDir(), blocks)
	if err != nil || len(diagrams) != 1 || diagrams[0].Image != "" {
		t.Fatalf("renderDiagrams = %+v, %v; want one source diagram and no error", diagrams, err)
	}
	notice := diagrams[0].Notice
	if !strings.Contains(notice, "Graphviz DOT") || !strings.Contains(notice, "dot was not found") || !strings.Contains(notice, DotEnv) {
		t.Fatalf("notice does not name the tool and its variable: %q", notice)
	}
	page := documentHTML(blocks, artwork{diagrams: diagrams}, Options{})
	for _, want := range []string{
		`<figure class="dot"><p class="notice"><em>` + notice + `</em></p>`,
		"<pre>// kind: action\ndigraph &#34;a &amp; b&#34; {\n  &#34;n0&#34; -&gt; &#34;n1&#34;;\n}</pre></figure>",
		`<p class="caption"><em>Figure 1. Flow</em></p>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("HTML missing %q:\n%s", want, page)
		}
	}
	if strings.Contains(page, "<img") {
		t.Errorf("a DOT block became an image:\n%s", page)
	}
	md := markdownWithImages(dotMarkdown, diagrams)
	if !strings.Contains(md, "*"+notice+"*\n\n```dot\n// kind: action\n") || !strings.Contains(md, "\n}\n```\n") {
		t.Errorf("Markdown lacks the notice ahead of the fence:\n%s", md)
	}
	if strings.Contains(md, "![diagram]") {
		t.Errorf("a DOT fence became an image reference:\n%s", md)
	}
}

// A PlantUML fence parses to its own block; without the jar it is kept as
// source under a notice naming the variable to set, on both converter inputs.
func TestPlantUMLBlockWithoutJarIsKeptAsSource(t *testing.T) {
	blocks, err := parseBlocks(plantumlMarkdown)
	if err != nil {
		t.Fatalf("parseBlocks: %v", err)
	}
	if len(blocks) != 4 || blocks[2].Kind != blockPlantUML || !strings.Contains(blocks[2].Source, "n0 --> n1") {
		t.Fatalf("blocks = %+v", blocks)
	}
	withoutDiagramTools(t)
	diagrams, err := renderDiagrams(t.TempDir(), blocks)
	if err != nil || len(diagrams) != 1 || diagrams[0].Image != "" {
		t.Fatalf("renderDiagrams = %+v, %v; want one source diagram and no error", diagrams, err)
	}
	notice := diagrams[0].Notice
	if !strings.Contains(notice, "PlantUML") || !strings.Contains(notice, "the PlantUML jar was not found") || !strings.Contains(notice, PlantUMLJarEnv) {
		t.Fatalf("notice does not name the jar and its variable: %q", notice)
	}
	page := documentHTML(blocks, artwork{diagrams: diagrams}, Options{})
	for _, want := range []string{
		`<figure class="plantuml"><p class="notice"><em>` + notice + `</em></p>`,
		"<pre>@startuml\n&#39; action rendering\n&lt;style&gt;\n",
		"state &#34;**a &amp; b**&#34; as n0 &lt;&lt;action&gt;&gt;\nn0 --&gt; n1\n@enduml</pre></figure>",
		`<p class="caption"><em>Figure 1. Flow</em></p>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("HTML missing %q:\n%s", want, page)
		}
	}
	if strings.Contains(page, "<img") || strings.Contains(page, `class="dot"`) {
		t.Errorf("a PlantUML block became an image or a DOT figure:\n%s", page)
	}
	md := markdownWithImages(plantumlMarkdown, diagrams)
	if !strings.Contains(md, "*"+notice+"*\n\n```plantuml\n@startuml\n") || !strings.Contains(md, "\n@enduml\n```\n") {
		t.Errorf("Markdown lacks the notice ahead of the fence:\n%s", md)
	}
	if strings.Contains(md, "![diagram]") {
		t.Errorf("a PlantUML fence became an image reference:\n%s", md)
	}
}

// With the jar but no java, a PlantUML block is kept as source under a notice
// that names java's variable; a jar path naming no file is the jar missing.
func TestPlantUMLNoticeNamesTheMissingPiece(t *testing.T) {
	blocks, err := parseBlocks(plantumlMarkdown)
	if err != nil {
		t.Fatalf("parseBlocks: %v", err)
	}
	withoutDiagramTools(t)
	dir := t.TempDir()
	jar := filepath.Join(dir, "plantuml.jar")
	if err := os.WriteFile(jar, []byte("PK"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(PlantUMLJarEnv, jar)
	diagrams, err := renderDiagrams(dir, blocks)
	if err != nil || len(diagrams) != 1 || !strings.Contains(diagrams[0].Notice, "java was not found") || !strings.Contains(diagrams[0].Notice, JavaEnv) {
		t.Fatalf("without java: %+v, %v", diagrams, err)
	}
	t.Setenv(PlantUMLJarEnv, filepath.Join(dir, "absent.jar"))
	fakeSVGTool(t, dir, "java", JavaEnv)
	diagrams, err = renderDiagrams(dir, blocks)
	if err != nil || len(diagrams) != 1 || !strings.Contains(diagrams[0].Notice, "absent.jar was not found") || !strings.Contains(diagrams[0].Notice, PlantUMLJarEnv) {
		t.Fatalf("with a jar path naming no file: %+v, %v", diagrams, err)
	}
}

// Rendering a document whose only diagram is DOT needs no Mermaid CLI and,
// without Graphviz, hands the converter the source under the notice.
func TestRenderDOTOnlyDocumentWithoutGraphviz(t *testing.T) {
	dir := t.TempDir()
	withoutDiagramTools(t)
	seenPath := fakeWeasyPrint(t, dir)
	pdf, err := Render(dotMarkdown, "weasyprint", Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF-") {
		t.Fatalf("output is no PDF: %q", pdf)
	}
	seen, err := os.ReadFile(seenPath)
	if err != nil {
		t.Fatalf("converter input: %v", err)
	}
	if !strings.Contains(string(seen), DotEnv) || !strings.Contains(string(seen), "digraph &#34;a &amp; b&#34;") {
		t.Fatalf("converter input lacks the DOT notice or source:\n%s", seen)
	}
}

// Rendering a document whose only diagram is PlantUML needs no Mermaid CLI
// and, without the jar, hands the converter the source under the notice.
func TestRenderPlantUMLOnlyDocumentWithoutJar(t *testing.T) {
	dir := t.TempDir()
	withoutDiagramTools(t)
	seenPath := fakeWeasyPrint(t, dir)
	pdf, err := Render(plantumlMarkdown, "weasyprint", Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF-") {
		t.Fatalf("output is no PDF: %q", pdf)
	}
	seen, err := os.ReadFile(seenPath)
	if err != nil {
		t.Fatalf("converter input: %v", err)
	}
	if !strings.Contains(string(seen), PlantUMLJarEnv) || !strings.Contains(string(seen), "n0 --&gt; n1") {
		t.Fatalf("converter input lacks the PlantUML notice or source:\n%s", seen)
	}
}

// With Graphviz, a DOT block is drawn to SVG and referenced as an image on
// both converter inputs; dot is run with -Tsvg on the block's source.
func TestRenderDOTWithFakeGraphviz(t *testing.T) {
	dir := t.TempDir()
	withoutDiagramTools(t)
	log := fakeSVGTool(t, dir, "dot", DotEnv)
	seenPath := fakeWeasyPrint(t, dir)
	if _, err := Render(dotMarkdown, "weasyprint", Options{}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	seen, _ := os.ReadFile(seenPath)
	if !strings.Contains(string(seen), `<figure><img src="diagram-1.svg" alt="diagram"></figure>`) || strings.Contains(string(seen), "did not draw") {
		t.Fatalf("converter input lacks the drawn diagram:\n%s", seen)
	}
	args, _ := os.ReadFile(log)
	if !strings.Contains(string(args), "args:-Kdot -Tsvg -o diagram-1.svg diagram-1.dot") {
		t.Fatalf("dot arguments: %s", args)
	}
	blocks, err := parseBlocks(dotMarkdown)
	if err != nil {
		t.Fatalf("parseBlocks: %v", err)
	}
	diagrams, err := renderDiagrams(dir, blocks)
	if err != nil {
		t.Fatalf("renderDiagrams: %v", err)
	}
	md := markdownWithImages(dotMarkdown, diagrams)
	if strings.Contains(md, "```dot") || !strings.Contains(md, "![diagram](diagram-1.svg)") {
		t.Fatalf("Markdown keeps the fence or lacks the image:\n%s", md)
	}
	source, err := os.ReadFile(filepath.Join(dir, "diagram-1.dot"))
	if err != nil || !strings.HasPrefix(string(source), "// kind: action\ndigraph \"a & b\"") {
		t.Fatalf("dot input: %q, %v", source, err)
	}
}

// The `// layout:` header the DOT writer opens a block with picks the layout
// engine, and `-n` keeps the positions the block states.
func TestGraphvizLayoutArgs(t *testing.T) {
	for source, want := range map[string]string{
		"digraph G {}":                                                "-Kdot",
		"// kind: action\ndigraph G {}":                               "-Kdot",
		"// layout: dot\ndigraph G {}":                                "-Kdot",
		"// layout: neato\ndigraph G {}":                              "-Kneato",
		"// layout: neato -n\ndigraph G {}":                           "-Kneato -n",
		"// layout: neato -n2\ndigraph G {}":                          "-Kneato -n2",
		"// kind: interconnection\n// layout: neato -n\n\ngraph G {}": "-Kneato -n",
		"// layout: fdp\ngraph G {}":                                  "-Kfdp",
		"// layout: neato -Gsplines=true\ndigraph G {}":               "-Kdot",
		"// layout: cat /etc/passwd\ndigraph G {}":                    "-Kdot",
		"digraph G {\n// layout: neato\n}":                            "-Kdot",
	} {
		if got := strings.Join(layoutArgs(source), " "); got != want {
			t.Errorf("%q: got %q, want %q", source, got, want)
		}
	}
}

// A positioned block runs the engine its header names.
func TestRenderDOTRunsTheHeaderEngine(t *testing.T) {
	dir := t.TempDir()
	withoutDiagramTools(t)
	log := fakeSVGTool(t, dir, "dot", DotEnv)
	md := "# T\n\n```dot\n// kind: interconnection\n// layout: neato -n\ngraph G {\n  a [pos=\"0,0\"];\n}\n```\n"
	blocks, err := parseBlocks(md)
	if err != nil {
		t.Fatalf("parseBlocks: %v", err)
	}
	if _, err := renderDiagrams(dir, blocks); err != nil {
		t.Fatalf("renderDiagrams: %v", err)
	}
	args, _ := os.ReadFile(log)
	if !strings.Contains(string(args), "args:-Kneato -n -Tsvg -o diagram-1.svg diagram-1.dot") {
		t.Fatalf("dot arguments: %s", args)
	}
}

// With java and the jar, a PlantUML block is drawn through the jar in pipe
// mode: the source on stdin, the SVG from stdout into the diagram file.
func TestRenderPlantUMLWithFakeJava(t *testing.T) {
	dir := t.TempDir()
	withoutDiagramTools(t)
	jar := filepath.Join(dir, "plantuml.jar")
	if err := os.WriteFile(jar, []byte("PK"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(PlantUMLJarEnv, jar)
	log := fakeSVGTool(t, dir, "java", JavaEnv)
	seenPath := fakeWeasyPrint(t, dir)
	if _, err := Render(plantumlMarkdown, "weasyprint", Options{}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	seen, _ := os.ReadFile(seenPath)
	if !strings.Contains(string(seen), `<figure><img src="diagram-1.svg" alt="diagram"></figure>`) || strings.Contains(string(seen), "did not draw") {
		t.Fatalf("converter input lacks the drawn diagram:\n%s", seen)
	}
	logged, _ := os.ReadFile(log)
	if !strings.Contains(string(logged), "args:-Djava.awt.headless=true -jar "+jar+" -tsvg -pipe\n") {
		t.Fatalf("java arguments: %s", logged)
	}
	if !strings.Contains(string(logged), "@startuml\n' action rendering\n") || !strings.Contains(string(logged), "n0 --> n1\n@enduml\n") {
		t.Fatalf("source not piped to the jar: %s", logged)
	}
	blocks, err := parseBlocks(plantumlMarkdown)
	if err != nil {
		t.Fatalf("parseBlocks: %v", err)
	}
	diagrams, err := renderDiagrams(dir, blocks)
	if err != nil {
		t.Fatalf("renderDiagrams: %v", err)
	}
	svg, err := os.ReadFile(filepath.Join(dir, diagrams[0].Image))
	if err != nil || !strings.Contains(string(svg), "drawn by java") {
		t.Fatalf("SVG from stdout: %q, %v", svg, err)
	}
	md := markdownWithImages(plantumlMarkdown, diagrams)
	if strings.Contains(md, "```plantuml") || !strings.Contains(md, "![diagram](diagram-1.svg)") {
		t.Fatalf("Markdown keeps the fence or lacks the image:\n%s", md)
	}
}

// The jar and the tools may be named by paths relative to the working
// directory; they are still found when the tools run in the render directory.
func TestRenderPlantUMLWithRelativeJarAndJava(t *testing.T) {
	dir := t.TempDir()
	withoutDiagramTools(t)
	if err := os.WriteFile(filepath.Join(dir, "plantuml.jar"), []byte("PK"), 0o600); err != nil {
		t.Fatal(err)
	}
	log := fakeSVGTool(t, dir, "java", JavaEnv)
	fakeWeasyPrint(t, dir)
	t.Chdir(dir)
	t.Setenv(PlantUMLJarEnv, "plantuml.jar")
	t.Setenv(JavaEnv, "./java")
	if _, err := Render(plantumlMarkdown, "weasyprint", Options{}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	logged, _ := os.ReadFile(log)
	if !strings.Contains(string(logged), "-jar "+filepath.Join(dir, "plantuml.jar")+" -tsvg -pipe\n") {
		t.Fatalf("java was not given the jar's absolute path: %s", logged)
	}
}

// A Graphviz or PlantUML that is installed but fails is the typed error a
// failing Mermaid CLI is, carrying the tool's stderr.
func TestRenderDiagramToolFailed(t *testing.T) {
	failing := `echo "syntax error in line 2 near '->'" >&2
exit 1
`
	dir := t.TempDir()
	withoutDiagramTools(t)
	fakeWeasyPrint(t, dir)
	fakeTool(t, dir, "dot", DotEnv, failing)
	_, err := Render(dotMarkdown, "weasyprint", Options{})
	var docErr *Error
	if !errors.As(err, &docErr) || docErr.Kind != ErrorToolFailed || docErr.Tool != "dot" {
		t.Fatalf("failing dot: got %v, want ErrorToolFailed from dot", err)
	}
	if !strings.Contains(docErr.Detail, "syntax error in line 2") {
		t.Fatalf("dot stderr not carried: %q", docErr.Detail)
	}

	jar := filepath.Join(dir, "plantuml.jar")
	if err := os.WriteFile(jar, []byte("PK"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(PlantUMLJarEnv, jar)
	fakeTool(t, dir, "java", JavaEnv, `echo "INFO: Created user preferences directory." >&2
echo "ERROR" >&2
echo "2" >&2
echo "Syntax Error? (Assumed diagram type: class)" >&2
exit 200
`)
	_, err = Render(plantumlMarkdown, "weasyprint", Options{})
	if !errors.As(err, &docErr) || docErr.Kind != ErrorToolFailed || docErr.Tool != "java" {
		t.Fatalf("failing PlantUML: got %v, want ErrorToolFailed from java", err)
	}
	if !strings.Contains(docErr.Detail, "Syntax Error?") || strings.Contains(docErr.Detail, "INFO:") {
		t.Fatalf("PlantUML stderr not carried, or the JVM's notes kept: %q", docErr.Detail)
	}
}

// outputArg is the shell prologue of a fake dot that reads its `-o` argument into $out.
const outputArg = `out=""
while [ $# -gt 0 ]; do case "$1" in -o) out="$2"; shift ;; esac; shift; done
`

// A tool that exits 0 without writing an SVG document — nothing, its
// diagnostics, malformed XML or some other document — is a failure too.
func TestRenderDiagramToolWroteNoSVG(t *testing.T) {
	cases := map[string]string{
		"nothing":      "exit 0\n",
		"diagnostics":  `printf 'warning: renderer unavailable\n' > "$out"`,
		"malformedXML": `printf '<svg xmlns="http://www.w3.org/2000/svg"><text>unclosed' > "$out"`,
		"html":         `printf '<html><body>not a drawing</body></html>' > "$out"`,
		"noNamespace":  `printf '<svg><text>x</text></svg>' > "$out"`,
		"secondRoot":   `printf '<svg xmlns="http://www.w3.org/2000/svg"/><html/>' > "$out"`,
		"textBefore":   `printf 'warning: font missing\n<svg xmlns="http://www.w3.org/2000/svg"/>' > "$out"`,
		"textAfter":    `printf '<svg xmlns="http://www.w3.org/2000/svg"/>\nwarning: font missing\n' > "$out"`,
	}
	for name, script := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			withoutDiagramTools(t)
			fakeTool(t, dir, "dot", DotEnv, outputArg+script+"\n")
			blocks, err := parseBlocks(dotMarkdown)
			if err != nil {
				t.Fatalf("parseBlocks: %v", err)
			}
			_, err = renderDiagrams(dir, blocks)
			var docErr *Error
			if !errors.As(err, &docErr) || docErr.Kind != ErrorToolFailed || docErr.Tool != "dot" || !strings.Contains(docErr.Detail, "wrote no SVG") || !strings.Contains(docErr.Detail, "diagram 1") {
				t.Fatalf("got %v, want ErrorToolFailed from dot naming diagram 1", err)
			}
		})
	}
}

// A real drawing opens on an XML declaration and a DOCTYPE before its root,
// as Graphviz and PlantUML write it, and is accepted.
func TestRenderDiagramToolWroteAPrefacedSVG(t *testing.T) {
	dir := t.TempDir()
	withoutDiagramTools(t)
	fakeTool(t, dir, "dot", DotEnv, outputArg+`printf '<?xml version="1.0" encoding="UTF-8" standalone="no"?>\n<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd">\n<!-- Generated by graphviz -->\n<svg width="8pt" height="8pt" xmlns="http://www.w3.org/2000/svg"><g/></svg>\n' > "$out"`+"\n")
	blocks, err := parseBlocks(dotMarkdown)
	if err != nil {
		t.Fatalf("parseBlocks: %v", err)
	}
	drawn, err := renderDiagrams(dir, blocks)
	if err != nil {
		t.Fatalf("renderDiagrams: %v", err)
	}
	if len(drawn) != 1 || drawn[0].Image != "diagram-1.svg" {
		t.Fatalf("got %+v, want the one drawn image", drawn)
	}
}

// A document mixing every diagram form draws each with its tool, numbering
// the images in block order, and keeps as source only the blocks whose
// optional tool is absent.
func TestMixedDiagramFormsInBlockOrder(t *testing.T) {
	md := sampleMarkdown + "\n" + strings.TrimPrefix(dotMarkdown, "# T\n") + "\n" + strings.TrimPrefix(plantumlMarkdown, "# T\n")
	blocks, err := parseBlocks(md)
	if err != nil {
		t.Fatalf("parseBlocks: %v", err)
	}
	dir := t.TempDir()
	withoutDiagramTools(t)
	fakeSVGTool(t, dir, "mmdc", MermaidEnv)
	fakeSVGTool(t, dir, "dot", DotEnv)
	diagrams, err := renderDiagrams(dir, blocks)
	if err != nil {
		t.Fatalf("renderDiagrams: %v", err)
	}
	if len(diagrams) != 3 || diagrams[0].Image != "diagram-1.svg" || diagrams[1].Image != "diagram-2.svg" || diagrams[2].Image != "" || !strings.Contains(diagrams[2].Notice, PlantUMLJarEnv) {
		t.Fatalf("diagrams = %+v", diagrams)
	}
	page := documentHTML(blocks, artwork{diagrams: diagrams}, Options{})
	first, second, third := strings.Index(page, `<img src="diagram-1.svg"`), strings.Index(page, `<img src="diagram-2.svg"`), strings.Index(page, `<figure class="plantuml">`)
	if first < 0 || second < 0 || third < 0 || first > second || second > third {
		t.Fatalf("images at %d, %d, PlantUML at %d:\n%s", first, second, third, page)
	}
	if strings.Contains(page, `<figure class="dot">`) {
		t.Fatalf("the drawn DOT block was kept as source:\n%s", page)
	}
	out := markdownWithImages(md, diagrams)
	if strings.Contains(out, "```mermaid") || strings.Contains(out, "```dot") || !strings.Contains(out, "![diagram](diagram-1.svg)") ||
		!strings.Contains(out, "![diagram](diagram-2.svg)") || !strings.Contains(out, "```plantuml\n") {
		t.Fatalf("mixed Markdown:\n%s", out)
	}
	if strings.Index(out, "![diagram](diagram-1.svg)") > strings.Index(out, "![diagram](diagram-2.svg)") {
		t.Fatalf("images out of order:\n%s", out)
	}
}

// A missing Mermaid CLI stays an error: only Graphviz and PlantUML are optional.
func TestMermaidStaysRequiredBesideOptionalTools(t *testing.T) {
	dir := t.TempDir()
	withoutDiagramTools(t)
	fakeSVGTool(t, dir, "dot", DotEnv)
	blocks, err := parseBlocks(sampleMarkdown + "\n" + strings.TrimPrefix(dotMarkdown, "# T\n"))
	if err != nil {
		t.Fatalf("parseBlocks: %v", err)
	}
	_, err = renderDiagrams(dir, blocks)
	var docErr *Error
	if !errors.As(err, &docErr) || docErr.Kind != ErrorToolMissing || docErr.EnvVar != MermaidEnv {
		t.Fatalf("got %v, want ErrorToolMissing for mmdc", err)
	}
}
