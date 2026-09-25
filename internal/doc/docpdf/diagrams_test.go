package docpdf

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

// withoutDiagramTools points every diagram tool's variable at a file that does
// not exist, so a test sees the tools absent whatever the machine has installed.
func withoutDiagramTools(t *testing.T) {
	t.Helper()
	missing := t.TempDir()
	for _, env := range []string{MermaidEnv, DotEnv, JavaEnv, PlantUMLJarEnv} {
		t.Setenv(env, filepath.Join(missing, "no-"+strings.ToLower(env)+"-here"))
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

// fakeJar writes an empty stand-in jar in dir and points PlantUMLJarEnv at it.
func fakeJar(t *testing.T, dir string) string {
	t.Helper()
	jar := filepath.Join(dir, "plantuml.jar")
	if err := os.WriteFile(jar, []byte("PK"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(PlantUMLJarEnv, jar)
	return jar
}

// telescopeDiagrams lists the telescope report's two diagrams written in form.
func telescopeDiagrams(t *testing.T, form view.Form) []docrender.Diagram {
	t.Helper()
	diagrams, err := docrender.Diagrams(telescopeDocument(t), form, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(diagrams) != 2 {
		t.Fatalf("telescope report has %d diagrams, want 2", len(diagrams))
	}
	return diagrams
}

// TestSourceNoticesNameTheVariables checks the notice over a diagram kept as
// source names the tool that was not found and the variable that locates it.
func TestSourceNoticesNameTheVariables(t *testing.T) {
	if !strings.Contains(dotNotice, "Graphviz DOT") || !strings.Contains(dotNotice, "Graphviz was not found") || !strings.Contains(dotNotice, DotEnv) {
		t.Fatalf("DOT notice: %q", dotNotice)
	}
	if !strings.Contains(plantumlNotice, "PlantUML") || !strings.Contains(plantumlNotice, PlantUMLJarEnv) || !strings.Contains(plantumlNotice, JavaEnv) {
		t.Fatalf("PlantUML notice: %q", plantumlNotice)
	}
	for _, notice := range []string{dotNotice, plantumlNotice} {
		if !strings.Contains(PrintStylesheet, `content: "`+notice+`";`) {
			t.Fatalf("print stylesheet does not set the notice %q", notice)
		}
	}
}

// TestRenderDOTWithFakeGraphviz checks a document asked for as DOT is drawn
// through dot with -Tsvg on each diagram's source, and the page shows the
// images in document order in place of the source.
func TestRenderDOTWithFakeGraphviz(t *testing.T) {
	dir := t.TempDir()
	withoutDiagramTools(t)
	log := fakeSVGTool(t, dir, "dot", DotEnv)
	capture := captureWeasyPrint(t, dir)
	if _, err := Render(telescopeDocument(t), "weasyprint", Options{DiagramForm: view.FormDot}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	page, listing := readCapture(t, capture)
	work := captureDir(t, capture)
	images := fileRefs(work, []string{"diagram-1.svg", "diagram-2.svg"})
	first, second := strings.Index(page, `<img src="`+images[0]+`"`), strings.Index(page, `<img src="`+images[1]+`"`)
	if first < 0 || second < 0 || first > second || strings.Contains(page, `<pre class="dot">`) {
		t.Fatalf("page does not show the two drawn diagrams in order:\n%s", page)
	}
	for _, file := range []string{"diagram-1.dot", "diagram-1.svg", "diagram-2.dot", "diagram-2.svg"} {
		if !strings.Contains(listing, file) {
			t.Fatalf("render directory lacks %s:\n%s", file, listing)
		}
	}
	args, _ := os.ReadFile(log)
	for _, want := range []string{"args:-Kdot -Tsvg -o diagram-1.svg diagram-1.dot\n", "args:-Kdot -Tsvg -o diagram-2.svg diagram-2.dot\n"} {
		if !strings.Contains(string(args), want) {
			t.Fatalf("dot arguments lack %q: %s", want, args)
		}
	}
}

// TestDrawDOTWritesTheDiagramSource checks dot is handed each diagram's source
// as the DOT backend writes it, and the images are named in diagram order.
func TestDrawDOTWritesTheDiagramSource(t *testing.T) {
	dir := t.TempDir()
	withoutDiagramTools(t)
	fakeSVGTool(t, dir, "dot", DotEnv)
	diagrams := telescopeDiagrams(t, view.FormDot)
	images, err := drawDiagrams(dir, diagrams, view.FormDot)
	if err != nil {
		t.Fatalf("drawDiagrams: %v", err)
	}
	if len(images) != 2 || images[0] != "diagram-1.svg" || images[1] != "diagram-2.svg" {
		t.Fatalf("images = %q", images)
	}
	for i, diagram := range diagrams {
		source, err := os.ReadFile(filepath.Join(dir, images[i][:len(images[i])-len(".svg")]+".dot"))
		if err != nil || string(source) != diagram.Source+"\n" {
			t.Fatalf("dot input %d: %q, %v; want the diagram's source", i+1, source, err)
		}
	}
}

// TestGraphvizLayoutArgs checks the `// layout:` header the DOT writer opens a
// block with picks the layout engine, and `-n` keeps the positions it states.
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

// TestDrawDOTRunsTheHeaderEngine checks a positioned diagram runs the engine
// its header names.
func TestDrawDOTRunsTheHeaderEngine(t *testing.T) {
	dir := t.TempDir()
	withoutDiagramTools(t)
	log := fakeSVGTool(t, dir, "dot", DotEnv)
	positioned := docrender.Diagram{Name: "placed", Source: "// kind: interconnection\n// layout: neato -n\ngraph G {\n  a [pos=\"0,0\"];\n}"}
	if _, err := drawDiagrams(dir, []docrender.Diagram{positioned}, view.FormDot); err != nil {
		t.Fatalf("drawDiagrams: %v", err)
	}
	args, _ := os.ReadFile(log)
	if !strings.Contains(string(args), "args:-Kneato -n -Tsvg -o diagram-1.svg diagram-1.dot") {
		t.Fatalf("dot arguments: %s", args)
	}
}

// TestRenderPlantUMLWithFakeJava checks a document asked for as PlantUML is
// drawn through the jar in pipe mode — the source on stdin, the SVG from
// stdout into the diagram file — and the page shows the images.
func TestRenderPlantUMLWithFakeJava(t *testing.T) {
	dir := t.TempDir()
	withoutDiagramTools(t)
	jar := fakeJar(t, dir)
	log := fakeSVGTool(t, dir, "java", JavaEnv)
	capture := captureWeasyPrint(t, dir)
	if _, err := Render(telescopeDocument(t), "weasyprint", Options{DiagramForm: view.FormPlantUML}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	page, listing := readCapture(t, capture)
	images := fileRefs(captureDir(t, capture), []string{"diagram-1.svg", "diagram-2.svg"})
	if !strings.Contains(page, `<img src="`+images[0]+`"`) || !strings.Contains(page, `<img src="`+images[1]+`"`) || strings.Contains(page, `<pre class="plantuml">`) {
		t.Fatalf("page does not show the two drawn diagrams:\n%s", page)
	}
	if !strings.Contains(listing, "diagram-1.svg") || !strings.Contains(listing, "diagram-2.svg") {
		t.Fatalf("render directory lacks the SVGs:\n%s", listing)
	}
	logged, _ := os.ReadFile(log)
	if strings.Count(string(logged), "args:-Djava.awt.headless=true -jar "+jar+" -tsvg -pipe\n") != 2 {
		t.Fatalf("java arguments: %s", logged)
	}
	if strings.Count(string(logged), "@startuml\n") != 2 || strings.Count(string(logged), "@enduml\n") != 2 {
		t.Fatalf("source not piped to the jar: %s", logged)
	}
}

// TestDrawPlantUMLKeepsStdoutAsTheImage checks the SVG the jar writes to
// stdout becomes the diagram file.
func TestDrawPlantUMLKeepsStdoutAsTheImage(t *testing.T) {
	dir := t.TempDir()
	withoutDiagramTools(t)
	fakeJar(t, dir)
	fakeSVGTool(t, dir, "java", JavaEnv)
	images, err := drawDiagrams(dir, telescopeDiagrams(t, view.FormPlantUML), view.FormPlantUML)
	if err != nil {
		t.Fatalf("drawDiagrams: %v", err)
	}
	svg, err := os.ReadFile(filepath.Join(dir, images[0]))
	if err != nil || !strings.Contains(string(svg), "drawn by java") {
		t.Fatalf("SVG from stdout: %q, %v", svg, err)
	}
}

// TestRenderPlantUMLWithRelativeJarAndJava checks the jar and the tools may
// be named by paths relative to the working directory, and are still found
// when the tools run in the render directory.
func TestRenderPlantUMLWithRelativeJarAndJava(t *testing.T) {
	dir := t.TempDir()
	withoutDiagramTools(t)
	if err := os.WriteFile(filepath.Join(dir, "plantuml.jar"), []byte("PK"), 0o600); err != nil {
		t.Fatal(err)
	}
	log := fakeSVGTool(t, dir, "java", JavaEnv)
	captureWeasyPrint(t, dir)
	document := telescopeDocument(t)
	t.Chdir(dir)
	t.Setenv(PlantUMLJarEnv, "plantuml.jar")
	t.Setenv(JavaEnv, "./java")
	if _, err := Render(document, "weasyprint", Options{DiagramForm: view.FormPlantUML}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	logged, _ := os.ReadFile(log)
	if !strings.Contains(string(logged), "-jar "+filepath.Join(dir, "plantuml.jar")+" -tsvg -pipe\n") {
		t.Fatalf("java was not given the jar's absolute path: %s", logged)
	}
}

// TestRenderPlantUMLWithoutJavaKeepsSource checks the jar alone, without a
// java to run it, keeps the diagrams as source under the notice.
func TestRenderPlantUMLWithoutJavaKeepsSource(t *testing.T) {
	dir := t.TempDir()
	withoutDiagramTools(t)
	fakeJar(t, dir)
	capture := captureWeasyPrint(t, dir)
	if _, err := Render(telescopeDocument(t), "weasyprint", Options{DiagramForm: view.FormPlantUML}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	page, listing := readCapture(t, capture)
	if strings.Count(page, `<pre class="plantuml">`) != 2 || strings.Contains(page, "<img") || strings.Contains(listing, "diagram-") {
		t.Fatalf("page without java:\n%s\n%s", page, listing)
	}
}

// TestRenderDiagramToolFailed checks a Graphviz or PlantUML that is installed
// but fails is the typed error a failing Mermaid CLI is, carrying its stderr.
func TestRenderDiagramToolFailed(t *testing.T) {
	dir := t.TempDir()
	withoutDiagramTools(t)
	captureWeasyPrint(t, dir)
	fakeTool(t, dir, "dot", DotEnv, `echo "syntax error in line 2 near '->'" >&2
exit 1
`)
	_, err := Render(telescopeDocument(t), "weasyprint", Options{DiagramForm: view.FormDot})
	var docErr *Error
	if !errors.As(err, &docErr) || docErr.Kind != ErrorToolFailed || docErr.Tool != "dot" {
		t.Fatalf("failing dot: got %v, want ErrorToolFailed from dot", err)
	}
	if !strings.Contains(docErr.Detail, "syntax error in line 2") {
		t.Fatalf("dot stderr not carried: %q", docErr.Detail)
	}

	fakeJar(t, dir)
	fakeTool(t, dir, "java", JavaEnv, `echo "INFO: Created user preferences directory." >&2
echo "ERROR" >&2
echo "2" >&2
echo "Syntax Error? (Assumed diagram type: class)" >&2
exit 200
`)
	_, err = Render(telescopeDocument(t), "weasyprint", Options{DiagramForm: view.FormPlantUML})
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

// TestDrawDiagramToolWroteNoSVG checks a tool that exits 0 without writing an
// SVG document — nothing, diagnostics, malformed XML, another document, text
// around the root — is a failure too.
func TestDrawDiagramToolWroteNoSVG(t *testing.T) {
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
			_, err := drawDiagrams(dir, telescopeDiagrams(t, view.FormDot), view.FormDot)
			var docErr *Error
			if !errors.As(err, &docErr) || docErr.Kind != ErrorToolFailed || docErr.Tool != "dot" || !strings.Contains(docErr.Detail, "wrote no SVG") || !strings.Contains(docErr.Detail, "diagram 1") {
				t.Fatalf("got %v, want ErrorToolFailed from dot naming diagram 1", err)
			}
		})
	}
}

// TestDrawDiagramToolWroteAPrefacedSVG checks a drawing opening on an XML
// declaration and a DOCTYPE, as Graphviz and PlantUML write it, is accepted.
func TestDrawDiagramToolWroteAPrefacedSVG(t *testing.T) {
	dir := t.TempDir()
	withoutDiagramTools(t)
	fakeTool(t, dir, "dot", DotEnv, outputArg+`printf '<?xml version="1.0" encoding="UTF-8" standalone="no"?>\n<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd">\n<!-- Generated by graphviz -->\n<svg width="8pt" height="8pt" xmlns="http://www.w3.org/2000/svg"><g/></svg>\n' > "$out"`+"\n")
	images, err := drawDiagrams(dir, telescopeDiagrams(t, view.FormDot)[:1], view.FormDot)
	if err != nil {
		t.Fatalf("drawDiagrams: %v", err)
	}
	if len(images) != 1 || images[0] != "diagram-1.svg" {
		t.Fatalf("got %q, want the one drawn image", images)
	}
}

// TestDrawDiagramsWithoutDiagrams checks a document with nothing to draw
// looks for no tool.
func TestDrawDiagramsWithoutDiagrams(t *testing.T) {
	withoutDiagramTools(t)
	for _, form := range []view.Form{"", view.FormMermaid, view.FormDot, view.FormPlantUML} {
		images, err := drawDiagrams(t.TempDir(), nil, form)
		if err != nil || len(images) != 0 {
			t.Fatalf("%q: got %q, %v", form, images, err)
		}
	}
}

// TestMermaidStaysRequiredBesideOptionalTools checks a missing Mermaid CLI
// stays an error: only Graphviz and PlantUML are optional.
func TestMermaidStaysRequiredBesideOptionalTools(t *testing.T) {
	dir := t.TempDir()
	withoutDiagramTools(t)
	fakeSVGTool(t, dir, "dot", DotEnv)
	_, err := drawDiagrams(dir, telescopeDiagrams(t, view.FormMermaid), view.FormMermaid)
	var docErr *Error
	if !errors.As(err, &docErr) || docErr.Kind != ErrorToolMissing || docErr.EnvVar != MermaidEnv {
		t.Fatalf("got %v, want ErrorToolMissing for mmdc", err)
	}
}

// TestMermaidConfigFitsTheChart checks each Mermaid chart is drawn under a
// configuration of its own, with plain text labels and text and edge caps a
// chart larger than Mermaid's defaults (500 edges, 50 000 characters) fits under.
func TestMermaidConfigFitsTheChart(t *testing.T) {
	dir := t.TempDir()
	withoutDiagramTools(t)
	log := fakeSVGTool(t, dir, "mmdc", MermaidEnv)
	var b strings.Builder
	b.WriteString("flowchart TD\n")
	for i := 0; i < 600; i++ {
		fmt.Fprintf(&b, "  n%d[\"%s\"]\n  n0 --- n%d\n", i, strings.Repeat("x", 80), i)
	}
	large := docrender.Diagram{Name: "Large", Source: b.String()}
	small := docrender.Diagram{Name: "Small", Source: "flowchart TD\n  a --- b\n"}
	if _, err := drawDiagrams(dir, []docrender.Diagram{large, small}, view.FormMermaid); err != nil {
		t.Fatalf("drawDiagrams: %v", err)
	}
	args, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	for i, diagram := range []docrender.Diagram{large, small} {
		name := fmt.Sprintf("diagram-%d.json", i+1)
		if !strings.Contains(string(args), "--configFile "+name) {
			t.Fatalf("mmdc was not handed %s:\n%s", name, args)
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		var config mermaidConfig
		if err := json.Unmarshal(raw, &config); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		edges := strings.Count(diagram.Source, "---")
		if config.MaxEdges <= edges || config.MaxTextSize <= len(diagram.Source) {
			t.Errorf("%s caps %d edges and %d characters; the chart has %d and %d", name, config.MaxEdges, config.MaxTextSize, edges, len(diagram.Source))
		}
		if config.HTMLLabels || config.Flowchart.HTMLLabels || config.Class.HTMLLabels {
			t.Errorf("%s keeps HTML labels: %s", name, raw)
		}
	}
	if config := configFor(small.Source); config.MaxEdges > 10 {
		t.Errorf("a two-line chart is capped at %d edges, not sized to the chart", config.MaxEdges)
	}
}

// TestRenderForPandocDrawsDOTAndPlantUML checks the Markdown-reading engine
// is handed a filter swapping each fence for the image its tool drew.
func TestRenderForPandocDrawsDOTAndPlantUML(t *testing.T) {
	dir := t.TempDir()
	withoutDiagramTools(t)
	fakeSVGTool(t, dir, "dot", DotEnv)
	fakeJar(t, dir)
	fakeSVGTool(t, dir, "java", JavaEnv)
	capture := filepath.Join(dir, "capture")
	fakeTool(t, dir, "pandoc", PandocEnv,
		`pwd > "`+capture+`.dir"; cp "$(dirname "$1")/artwork.lua" "`+capture+`.lua"; out=""; while [ $# -gt 0 ]; do [ "$1" = "--output" ] && out="$2"; shift; done; printf '%%PDF-1.7 fake' > "$out"`+"\n")
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv, "exit 0\n")
	for _, form := range []view.Form{view.FormDot, view.FormPlantUML} {
		if _, err := Render(telescopeDocument(t), "pandoc", Options{DiagramForm: form}); err != nil {
			t.Fatalf("Render %s: %v", form, err)
		}
		filter, err := os.ReadFile(capture + ".lua")
		if err != nil {
			t.Fatal(err)
		}
		images := fileRefs(captureDir(t, capture), []string{"diagram-1.svg", "diagram-2.svg"})
		for _, want := range []string{`local form = "` + string(form) + `"`, `local images = {"` + images[0] + `", "` + images[1] + `"}`} {
			if !strings.Contains(string(filter), want) {
				t.Fatalf("%s filter lacks %q:\n%s", form, want, filter)
			}
		}
	}
}

// TestRenderDOTStyleReachesGraphviz checks the drawing style reaches the DOT
// figures a PDF draws: the page states it on each figure and dot is run with
// the Cameo source under `cameo`, with the Pilot look by default; a style
// there is none of is refused before any tool runs.
func TestRenderDOTStyleReachesGraphviz(t *testing.T) {
	dir := t.TempDir()
	withoutDiagramTools(t)
	fakeSVGTool(t, dir, "dot", DotEnv)
	capture := captureWeasyPrint(t, dir)
	for _, style := range []view.DrawingStyle{"", view.StyleCameo} {
		if _, err := Render(telescopeDocument(t), "weasyprint", Options{DiagramForm: view.FormDot, Style: style}); err != nil {
			t.Fatalf("Render(%q): %v", style, err)
		}
		page, _ := readCapture(t, capture)
		if strings.Contains(page, `data-style="cameo"`) != (style == view.StyleCameo) || strings.Contains(page, `<pre class="dot">`) {
			t.Fatalf("style %q page:\n%s", style, page)
		}
	}
	diagrams, err := docrender.Diagrams(telescopeDocument(t), view.FormDot, "", view.StyleCameo)
	if err != nil {
		t.Fatal(err)
	}
	images, err := drawDiagrams(dir, diagrams, view.FormDot)
	if err != nil {
		t.Fatalf("drawDiagrams: %v", err)
	}
	source, err := os.ReadFile(filepath.Join(dir, strings.TrimSuffix(images[0], ".svg")+".dot"))
	if err != nil || !strings.Contains(string(source), `subgraph "cluster_frame"`) || !strings.Contains(string(source), `fontname="Arial"`) {
		t.Fatalf("dot input under cameo: %v\n%s", err, source)
	}
	_, err = Render(telescopeDocument(t), "weasyprint", Options{DiagramForm: view.FormDot, Style: "magicdraw"})
	if err == nil || !strings.Contains(err.Error(), `unknown drawing style "magicdraw"`) {
		t.Fatalf("an unknown drawing style: %v", err)
	}
}
