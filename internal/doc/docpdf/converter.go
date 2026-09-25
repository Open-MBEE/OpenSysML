// Package docpdf renders an evaluated document to PDF by driving external
// converters as subprocesses; no PDF renderer is linked into the binary. An
// engine reading HTML is handed the HTML backend's markup with the print
// stylesheet; one reading Markdown is handed the Markdown backend's text.
package docpdf

import (
	"context"
	_ "embed" // for the //go:embed directives below
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/exec/hostcap"
)

// Environment variables that point each external tool's discovery at a
// specific executable, ahead of a PATH lookup by its default name.
const (
	PandocEnv     = "OPENSYSML_PANDOC"
	WeasyPrintEnv = "OPENSYSML_WEASYPRINT"
	PrinceEnv     = "OPENSYSML_PRINCE"
	MermaidEnv    = "OPENSYSML_MMDC"
	// MermaidPuppeteerEnv names an optional puppeteer configuration file
	// passed to mmdc, for environments whose browser needs launch flags.
	MermaidPuppeteerEnv = "OPENSYSML_MMDC_PUPPETEER"
	KatexEnv            = "OPENSYSML_KATEX"
	// KatexCSSEnv names KaTeX's stylesheet, with its fonts directory beside
	// it, when it is not in the dist directory of the katex executable.
	KatexCSSEnv = "OPENSYSML_KATEX_CSS"
	// DotEnv names the Graphviz dot executable that draws DOT blocks; absent,
	// they are kept as source.
	DotEnv = "OPENSYSML_DOT"
	// JavaEnv names the java that runs the PlantUML jar.
	JavaEnv = "OPENSYSML_JAVA"
	// PlantUMLJarEnv names the PlantUML jar that draws PlantUML blocks; absent,
	// they are kept as source.
	PlantUMLJarEnv = "OPENSYSML_PLANTUML_JAR"
)

// toolTimeout bounds each converter subprocess, so a wedged tool is a typed
// error rather than a hang.
const toolTimeout = 5 * time.Minute

// Prepared is one document laid out in a working directory for a converter,
// in the form the converter reads, with its diagrams drawn and its formulas
// typeset beside it.
type Prepared struct {
	// Dir is the working directory holding every input, diagram SVGs included.
	Dir string

	// MarkdownFile is the Markdown document's name within Dir, for a converter
	// reading Markdown; empty for one reading HTML.
	MarkdownFile string

	// Filter is a pandoc Lua filter within Dir that swaps the drawn diagrams
	// and typeset formulas into the Markdown; empty when there are none.
	Filter string

	// HTMLFile is the HTML document's name within Dir, for a converter reading
	// HTML; empty for one reading Markdown.
	HTMLFile string

	// MathCSS is the KaTeX stylesheet's path within Dir, for converters that
	// read the Markdown themselves; empty when the document has no formulas.
	MathCSS string

	// BaseDir is the absolute directory the document's relative references
	// resolve against; Dir's own files are referenced by absolute file URL.
	BaseDir string

	// Options are the deliverable choices, for converters with native flags.
	Options Options
}

// Input is the document form a converter reads.
type Input string

const (
	InputHTML     Input = "html"
	InputMarkdown Input = "markdown"
)

// Capabilities is what a converter states about itself: the form it reads,
// the executables it drives, and whether it applies the deliverable options
// natively rather than through the prepared HTML.
type Capabilities struct {
	Input         Input
	Tools         []string
	NativeOptions bool
}

// Converter is one external Markdown/HTML-to-PDF toolchain, run as a
// subprocess.
type Converter interface {
	// Name is what -pdf-engine selects the converter by.
	Name() string

	// Capabilities states the converter's input form and tools.
	Capabilities() Capabilities

	// Available returns nil, or a typed error naming the missing tool.
	Available() error

	// Convert lays the prepared document out as PDF bytes.
	Convert(doc *Prepared) ([]byte, error)
}

// Engines names the converters, in the order they are offered. The first is
// the default.
func Engines() []string { return []string{"weasyprint", "pandoc", "prince"} }

// EngineNamed returns the converter -pdf-engine named, the default for "",
// and a typed error for a name no converter answers to.
func EngineNamed(name string) (Converter, error) {
	if name == "" {
		name = Engines()[0]
	}
	switch name {
	case "weasyprint":
		return &weasyPrintConverter{}, nil
	case "pandoc":
		return &pandocConverter{}, nil
	case "prince":
		return &princeConverter{}, nil
	default:
		return nil, &Error{Kind: ErrorUnknownEngine, Engine: name, Engines: Engines()}
	}
}

// tool is one external executable a converter or the diagram renderer needs.
type tool struct {
	name   string // default executable name, looked up on PATH
	envVar string // environment variable naming a specific executable
}

var (
	pandocTool     = tool{name: "pandoc", envVar: PandocEnv}
	weasyPrintTool = tool{name: "weasyprint", envVar: WeasyPrintEnv}
	princeTool     = tool{name: "prince", envVar: PrinceEnv}
	mermaidTool    = tool{name: "mmdc", envVar: MermaidEnv}
	katexTool      = tool{name: "katex", envVar: KatexEnv}
	graphvizTool   = tool{name: "dot", envVar: DotEnv}
	javaTool       = tool{name: "java", envVar: JavaEnv}
)

// locate finds the tool via its environment override or a PATH lookup;
// engine names the converter looking ("" for the diagram renderer). The
// path comes back absolute, since the tool runs in the render directory.
func (t tool) locate(engine string) (string, error) {
	// The tool is an external process: on a host that starts none, that is why it
	// will not run here, answered before the lookup, whose advice to install it is
	// advice no such host could act on.
	subject := strings.TrimSpace(os.Getenv(t.envVar))
	if subject == "" {
		subject = t.name
	}
	if err := hostcap.CheckSpawn(subject); err != nil {
		return "", err
	}
	if override := strings.TrimSpace(os.Getenv(t.envVar)); override != "" {
		path, err := exec.LookPath(override)
		if err != nil {
			return "", &Error{Kind: ErrorToolMissing, Engine: engine, Tool: override, EnvVar: t.envVar}
		}
		return absolute(path)
	}
	path, err := exec.LookPath(t.name)
	if err != nil {
		return "", &Error{Kind: ErrorToolMissing, Engine: engine, Tool: t.name, EnvVar: t.envVar}
	}
	return absolute(path)
}

// absolute resolves a path the operator gave against the working directory.
func absolute(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("docpdf: resolving %q: %w", path, err)
	}
	return abs, nil
}

// runTool runs one external executable in dir with SOURCE_DATE_EPOCH pinned
// for determinism; a failure is a typed error carrying the tool's stderr.
func runTool(dir, path string, args ...string) error {
	return runToolWith(dir, path, tail, args...)
}

// runToolWith is runTool with detail choosing what of the tool's stderr the
// failure reports.
func runToolWith(dir, path string, detail func(stderr string) string, args ...string) error {
	return toolRun{dir: dir, path: path, args: args, detail: detail}.run()
}

// toolRun is one invocation of an external executable: its arguments, the
// variables added to its environment, and what it reads and writes.
type toolRun struct {
	dir    string
	path   string
	args   []string
	env    []string
	stdin  io.Reader
	stdout io.Writer
	detail func(stderr string) string
}

// run runs the tool in dir with SOURCE_DATE_EPOCH pinned for determinism; a
// failure is a typed error carrying the tool's stderr.
//
// The path comes from tool.locate, which refuses on a host that can start no
// process (internal/exec/hostcap) with the reason a reader can act on; a toolRun
// built from any other path would skip that answer and fail on the platform's own
// words instead.
func (r toolRun) run() error {
	ctx, cancel := context.WithTimeout(context.Background(), toolTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.path, r.args...) // #nosec G204 -- the path is the operator's own tool choice
	cmd.Dir = r.dir
	cmd.Env = append(append(os.Environ(), "SOURCE_DATE_EPOCH=0"), r.env...)
	cmd.Stdin = r.stdin
	cmd.Stdout = r.stdout
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		said := strings.TrimSpace(stderr.String())
		if said == "" {
			said = err.Error()
		}
		detail := r.detail
		if detail == nil {
			detail = tail
		}
		return &Error{Kind: ErrorToolFailed, Tool: filepath.Base(r.path), Detail: detail(said)}
	}
	return nil
}

// tail keeps the last lines of a tool's stderr, where the failure is said.
func tail(detail string) string {
	lines := strings.Split(detail, "\n")
	if len(lines) > 8 {
		lines = lines[len(lines)-8:]
	}
	return strings.Join(lines, "\n")
}

// readPDF reads the PDF a converter wrote, requiring the PDF signature.
func readPDF(dir, name, toolName string) ([]byte, error) {
	pdf, err := os.ReadFile(filepath.Clean(filepath.Join(dir, name)))
	if err != nil || !strings.HasPrefix(string(pdf), "%PDF-") {
		return nil, &Error{Kind: ErrorNoPDF, Tool: toolName}
	}
	return pdf, nil
}

// weasyPrintConverter lays the prepared HTML out with WeasyPrint, whose
// paged-media support carries the stylesheet's page numbers and breaks.
type weasyPrintConverter struct{}

func (*weasyPrintConverter) Name() string { return "weasyprint" }

func (*weasyPrintConverter) Capabilities() Capabilities {
	return Capabilities{Input: InputHTML, Tools: []string{weasyPrintTool.name}}
}

func (c *weasyPrintConverter) Available() error {
	_, err := weasyPrintTool.locate(c.Name())
	return err
}

func (c *weasyPrintConverter) Convert(doc *Prepared) ([]byte, error) {
	path, err := weasyPrintTool.locate(c.Name())
	if err != nil {
		return nil, err
	}
	if err := runTool(doc.Dir, path, doc.HTMLFile, outputName, "--base-url", dirURL(doc.BaseDir)); err != nil {
		return nil, err
	}
	return readPDF(doc.Dir, outputName, weasyPrintTool.name)
}

// pandocConverter lays the prepared Markdown out with pandoc, applying the
// options through pandoc's own flags, with WeasyPrint as its PDF engine.
type pandocConverter struct{}

func (*pandocConverter) Name() string { return "pandoc" }

func (*pandocConverter) Capabilities() Capabilities {
	return Capabilities{Input: InputMarkdown, Tools: []string{pandocTool.name, weasyPrintTool.name}, NativeOptions: true}
}

func (c *pandocConverter) Available() error {
	if _, err := pandocTool.locate(c.Name()); err != nil {
		return err
	}
	_, err := weasyPrintTool.locate(c.Name())
	return err
}

func (c *pandocConverter) Convert(doc *Prepared) ([]byte, error) {
	pandoc, err := pandocTool.locate(c.Name())
	if err != nil {
		return nil, err
	}
	engine, err := weasyPrintTool.locate(c.Name())
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(doc.Dir, pandocCSSName), []byte(pandocCSS(doc.Options)), 0o600); err != nil {
		return nil, err
	}
	// Shifting the title heading into pandoc's title block keeps it unnumbered.
	// pandoc embeds the page's resources itself, searching the later resource
	// path first, so a sheet's relative references resolve beside the PDF.
	args := []string{
		doc.MarkdownFile,
		"--from", "commonmark_x",
		"--to", "pdf",
		"--pdf-engine", engine,
		"--standalone",
		"--shift-heading-level-by", "-1",
		"--css", fileURL(filepath.Join(doc.Dir, pandocCSSName)),
		"--output", outputName,
		"--resource-path", ".",
		"--resource-path", doc.BaseDir,
		"--pdf-engine-opt=--base-url=" + dirURL(doc.BaseDir),
	}
	if doc.MathCSS != "" {
		args = append(args, "--css", fileURL(filepath.Join(doc.Dir, doc.MathCSS)))
	}
	if len(doc.Options.Stylesheets) > 0 {
		markup := docrender.StylesheetMarkup(doc.Options.Stylesheets)
		if err := os.WriteFile(filepath.Join(doc.Dir, readerSheetsName), []byte(markup), 0o600); err != nil {
			return nil, err
		}
		args = append(args, "--include-in-header", readerSheetsName)
	}
	if doc.Filter != "" {
		args = append(args, "--lua-filter", doc.Filter)
	}
	if doc.Options.TOC {
		args = append(args, "--toc")
	}
	if doc.Options.NumberSections {
		args = append(args, "--number-sections")
	}
	if err := runTool(doc.Dir, pandoc, args...); err != nil {
		return nil, err
	}
	return readPDF(doc.Dir, outputName, pandocTool.name)
}

// princeConverter lays the prepared HTML out with Prince, an alternative
// paged-media engine selected the same way.
type princeConverter struct{}

func (*princeConverter) Name() string { return "prince" }

func (*princeConverter) Capabilities() Capabilities {
	return Capabilities{Input: InputHTML, Tools: []string{princeTool.name}}
}

func (c *princeConverter) Available() error {
	_, err := princeTool.locate(c.Name())
	return err
}

func (c *princeConverter) Convert(doc *Prepared) ([]byte, error) {
	path, err := princeTool.locate(c.Name())
	if err != nil {
		return nil, err
	}
	if err := runTool(doc.Dir, path, doc.HTMLFile, "-o", outputName, "--baseurl="+dirURL(doc.BaseDir)); err != nil {
		return nil, err
	}
	return readPDF(doc.Dir, outputName, princeTool.name)
}

// outputName is where a converter writes the PDF within the working directory.
const outputName = "document.pdf"

// pandocCSSName is the stylesheet the pandoc converter writes for its engine.
const pandocCSSName = "pandoc.css"

// pandocStylesheet is the print stylesheet for pandoc's own HTML, which
// carries pandoc's structure rather than the HTML backend's classes.
//
//go:embed pandoc.css
var pandocStylesheet string

// pandocCSS is the print stylesheet for pandoc's own HTML, with a page of its
// own for pandoc's title block when asked for.
func pandocCSS(opts Options) string {
	css := pandocStylesheet
	if opts.TitlePage {
		css += "header#title-block-header { page-break-after: always; text-align: center; padding-top: 35%; }\n"
	}
	return css
}

// readerSheetsName is the head markup attaching the reader's stylesheets,
// included in pandoc's page after its own so they override it, inline sheets
// inlined so their relative references resolve against the page's base.
const readerSheetsName = "reader-stylesheets.html"
