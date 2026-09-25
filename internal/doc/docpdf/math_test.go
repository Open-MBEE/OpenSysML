package docpdf

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
)

// fakeKatex installs a katex stand-in that writes a recognizable span around
// the input, marking display mode, plus a stylesheet and fonts beside it as
// the real package lays them out.
func fakeKatex(t *testing.T, dir string) {
	t.Helper()
	pkg := filepath.Join(dir, "katex-pkg")
	if err := os.MkdirAll(filepath.Join(pkg, "dist", "fonts"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "dist", "katex.min.css"), []byte(".katex{font-family:KaTeX_Main}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "dist", "fonts", "KaTeX_Main-Regular.woff2"), []byte("font"), 0o600); err != nil {
		t.Fatal(err)
	}
	fakeTool(t, pkg, "katex", KatexEnv, strings.Join([]string{
		`mode=inline`,
		`while [ $# -gt 0 ]; do`,
		`  case "$1" in`,
		`    --input) in="$2"; shift 2;;`,
		`    --output) out="$2"; shift 2;;`,
		`    --display-mode) mode=display; shift;;`,
		`    *) shift;;`,
		`  esac`,
		`done`,
		`printf '<span class="katex %s">%s</span>' "$mode" "$(cat "$in")" > "$out"`,
		"",
	}, "\n"))
	t.Setenv(KatexCSSEnv, "")
}

// TestRenderFormulasWithFakeTools checks the whole preparation for an
// HTML-reading converter: KaTeX run once per distinct formula on the TeX the
// Markdown backend writes, its stylesheet and fonts copied beside the page,
// and the page showing the typeset HTML in the backend's math markup rather
// than LaTeX.
func TestRenderFormulasWithFakeTools(t *testing.T) {
	dir := t.TempDir()
	fakeKatex(t, dir)
	capture := captureWeasyPrint(t, dir)
	pdf, err := Render(mathDocument(t), "weasyprint", Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF-") {
		t.Fatalf("output is no PDF: %q", pdf)
	}
	page, listing := readCapture(t, capture)
	for _, want := range []string{
		`<link rel="stylesheet" href="` + fileURL(filepath.Join(captureDir(t, capture), "katex", "katex.min.css")) + `">`,
		`The mirror&#39;s mass scales as <span class="sysml-math"><span class="katex inline">m \propto D^{2.5}_{\text{eff}}</span></span> and each $ of budget`,
		"<div class=\"sysml-math\"><span class=\"katex display\">A = \\pi \\left(\\frac{D}{2}\\right)^2\n= \\frac{\\pi D^2}{4}</span></div>",
		`<div class="sysml-math"><span class="katex display">\text{cost} = 10^6\,\$ \times D^{2.5} + \$</span></div>`,
		`Rayleigh criterion <span class="sysml-math"><span class="katex inline">\theta = 1.22\,\frac{\lambda}{D}</span></span></li>`,
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("HTML missing %q:\n%s", want, page)
		}
	}
	for _, stray := range []string{`\(`, `\[`} {
		if strings.Contains(page, stray) {
			t.Fatalf("page leaks a formula as LaTeX %q:\n%s", stray, page)
		}
	}
	if strings.Count(page, "@layer opensysml-print {") != 1 {
		t.Fatalf("print stylesheet missing:\n%s", page)
	}
	if strings.Index(page, "@layer opensysml-print {") > strings.Index(page, "katex/katex.min.css") {
		t.Fatalf("KaTeX stylesheet ahead of the print stylesheet:\n%s", page)
	}
	for _, want := range []string{"katex.min.css", "KaTeX_Main-Regular.woff2", "formula-1.tex", "formula-5.html"} {
		if !strings.Contains(listing, want) {
			t.Fatalf("working directory lacks %s:\n%s", want, listing)
		}
	}
	if strings.Contains(listing, "formula-6.tex") {
		t.Fatalf("more formulas typeset than the document has:\n%s", listing)
	}
}

// TestRenderFormulasForPandoc checks the Markdown-reading path: the KaTeX
// stylesheet passed as a second --css, the Markdown untouched, and the
// filter carrying every formula's typeset HTML keyed by its TeX.
func TestRenderFormulasForPandoc(t *testing.T) {
	dir := t.TempDir()
	fakeKatex(t, dir)
	capture := filepath.Join(dir, "capture")
	fakeTool(t, dir, "pandoc", PandocEnv,
		`echo "$@" > "`+capture+`.args"; pwd > "`+capture+`.dir"; cp "$1" "`+capture+`.md"; cp "$(dirname "$1")/artwork.lua" "`+capture+`.lua"; out=""; while [ $# -gt 0 ]; do [ "$1" = "--output" ] && out="$2"; shift; done; printf '%%PDF-1.7 fake' > "$out"`+"\n")
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv, "exit 0\n")
	document := mathDocument(t)
	if _, err := Render(document, "pandoc", Options{}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	args, err := os.ReadFile(capture + ".args")
	if err != nil {
		t.Fatal(err)
	}
	work := captureDir(t, capture)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	wantArgs := "--css " + fileURL(filepath.Join(work, "pandoc.css")) + " --output document.pdf --resource-path . --resource-path " + cwd +
		" --pdf-engine-opt=--base-url=" + dirURL(cwd) + " --css " + fileURL(filepath.Join(work, "katex", "katex.min.css")) + " --lua-filter artwork.lua"
	if !strings.Contains(string(args), wantArgs) {
		t.Fatalf("pandoc arguments lack the KaTeX stylesheet and filter: %s", args)
	}
	md, err := os.ReadFile(capture + ".md")
	if err != nil {
		t.Fatal(err)
	}
	want, err := docrender.Markdown(document, docrender.MarkdownOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if string(md) != want {
		t.Fatalf("pandoc is handed other than the Markdown backend's text:\n%s", md)
	}
	filter, err := os.ReadFile(capture + ".lua")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`["inline:m \\propto D^{2.5}_{\\text{eff}}"] = "<span class=\"katex inline\">m \\propto D^{2.5}_{\\text{eff}}</span>",`,
		`["display:A = \\pi \\left(\\frac{D}{2}\\right)^2\n= \\frac{\\pi D^2}{4}"] = "<span class=\"katex display\">`,
		`["display:\\text{cost} = 10^6\\,\\$ \\times D^{2.5} + \\$"] = `,
		`["inline:\\theta = 1.22\\,\\frac{\\lambda}{D}"] = `,
	} {
		if !strings.Contains(string(filter), want) {
			t.Fatalf("filter lacks %q:\n%s", want, filter)
		}
	}
}

// TestRenderWithoutFormulasNeedsNoKatex checks that a document with no math
// renders with katex absent, and the working directory holds no stylesheet.
func TestRenderWithoutFormulasNeedsNoKatex(t *testing.T) {
	dir := t.TempDir()
	capture := captureWeasyPrint(t, dir)
	t.Setenv(KatexEnv, filepath.Join(dir, "no-katex-here"))
	if _, err := Render(plainDocument(t), "weasyprint", Options{}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	page, listing := readCapture(t, capture)
	if strings.Contains(page, "<link") || strings.Contains(listing, "katex") {
		t.Fatalf("page:\n%s\nworking directory:\n%s", page, listing)
	}
}

// TestRenderFormulasKatexMissing checks the typed error when a document has
// formulas but no katex is installed, and when its stylesheet is missing.
func TestRenderFormulasKatexMissing(t *testing.T) {
	dir := t.TempDir()
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv, "exit 0\n")
	t.Setenv(KatexEnv, "")
	t.Setenv(KatexCSSEnv, "")
	document := mathDocument(t)
	_, err := Render(document, "weasyprint", Options{})
	var docErr *Error
	if !errors.As(err, &docErr) || docErr.Kind != ErrorToolMissing || docErr.Tool != "katex" || docErr.EnvVar != KatexEnv {
		t.Fatalf("got %v, want ErrorToolMissing for katex", err)
	}
	msg := err.Error()
	for _, want := range []string{"typesetting the document's formulas needs katex", "point OPENSYSML_KATEX at it"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message %q lacks %q", msg, want)
		}
	}
	if strings.Contains(msg, "-pdf-engine") {
		t.Fatalf("message offers another engine, which would not help: %q", msg)
	}

	fakeTool(t, dir, "katex", KatexEnv, "exit 0\n")
	_, err = Render(document, "weasyprint", Options{})
	if !errors.As(err, &docErr) || docErr.Kind != ErrorToolMissing || docErr.Tool != "katex.min.css" || docErr.EnvVar != KatexCSSEnv {
		t.Fatalf("got %v, want ErrorToolMissing for the stylesheet", err)
	}
	if !strings.Contains(err.Error(), "point OPENSYSML_KATEX_CSS at it") {
		t.Fatalf("message %q", err.Error())
	}

	t.Setenv(KatexCSSEnv, filepath.Join(dir, "nowhere.css"))
	_, err = Render(document, "weasyprint", Options{})
	if !errors.As(err, &docErr) || docErr.Kind != ErrorToolMissing || docErr.Tool != filepath.Join(dir, "nowhere.css") {
		t.Fatalf("got %v, want ErrorToolMissing for the named stylesheet", err)
	}
}

// TestRenderFormulasStylesheetOverride checks that KatexCSSEnv names the
// stylesheet to copy, fonts beside it, when katex lives elsewhere.
func TestRenderFormulasStylesheetOverride(t *testing.T) {
	dir := t.TempDir()
	fakeKatex(t, dir)
	css := filepath.Join(dir, "elsewhere", "katex.min.css")
	if err := os.MkdirAll(filepath.Join(dir, "elsewhere", "fonts"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(css, []byte(".katex{color:red}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "elsewhere", "fonts", "Other.woff2"), []byte("font"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(KatexCSSEnv, css)
	capture := filepath.Join(dir, "capture")
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv,
		`cat "$(dirname "$1")/katex/katex.min.css" > "`+capture+`.css"; ls "$(dirname "$1")/katex/fonts" > "`+capture+`.fonts"; printf '%%PDF-1.7 fake' > "$2"`+"\n")
	if _, err := Render(mathDocument(t), "weasyprint", Options{}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	copied, err := os.ReadFile(capture + ".css")
	if err != nil {
		t.Fatal(err)
	}
	fonts, err := os.ReadFile(capture + ".fonts")
	if err != nil {
		t.Fatal(err)
	}
	if string(copied) != ".katex{color:red}\n" || strings.TrimSpace(string(fonts)) != "Other.woff2" {
		t.Fatalf("copied %q with fonts %q", copied, fonts)
	}
}

// TestRenderFormulasKatexFails checks that malformed LaTeX surfaces as the
// parse error katex reports, without its stack trace.
func TestRenderFormulasKatexFails(t *testing.T) {
	dir := t.TempDir()
	fakeKatex(t, dir)
	fakeTool(t, filepath.Join(dir, "katex-pkg"), "katex", KatexEnv, strings.Join([]string{
		`printf '%s\n' 'ParseError: KaTeX parse error: Expected group after '"'"'\frac'"'"' at end of input: \frac{a}' >&2`,
		`printf '%s\n' '    at new ParseError (/x/katex.js:1:1)' >&2`,
		`printf '%s\n' '    at Parser.parseGroup (/x/katex.js:2:2)' >&2`,
		`exit 1`,
		"",
	}, "\n"))
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv, "exit 0\n")
	document := mathDocument(t)
	_, err := Render(document, "weasyprint", Options{})
	var docErr *Error
	if !errors.As(err, &docErr) || docErr.Kind != ErrorToolFailed || docErr.Tool != "katex" {
		t.Fatalf("got %v, want ErrorToolFailed for katex", err)
	}
	if docErr.Detail != `KaTeX parse error: Expected group after '\frac' at end of input: \frac{a}` {
		t.Fatalf("detail %q", docErr.Detail)
	}

	fakeTool(t, filepath.Join(dir, "katex-pkg"), "katex", KatexEnv, "exit 0\n")
	_, err = Render(document, "weasyprint", Options{})
	if !errors.As(err, &docErr) || docErr.Kind != ErrorToolFailed || !strings.Contains(docErr.Detail, "wrote no HTML for formula-1.tex") {
		t.Fatalf("got %v, want ErrorToolFailed for missing output", err)
	}
}
