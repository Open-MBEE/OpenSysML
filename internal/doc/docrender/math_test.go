package docrender

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMarkdownMathReportGolden locks the Markdown of a report mixing inline
// math, display formulas (captioned and bare, multi-line, dollar-laden) and
// query-backed math runs against a committed golden file.
func TestMarkdownMathReportGolden(t *testing.T) {
	got := renderFixtureDocument(t,
		filepath.Join("testdata", "math_report.sysml"),
		"Optics::OpticsReport")
	golden := filepath.Join("testdata", "math_report.golden.md")
	if *update {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatalf("update golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if got != string(want) {
		t.Errorf("rendered Markdown differs from %s (run with -update after intentional changes)\ngot:\n%s", golden, got)
	}
}

// TestMarkdownMathRuns spot-checks the math conventions: LaTeX verbatim in
// dollar delimiters, prose dollars escaped, and the formula's caption and
// anchor written like a table's.
func TestMarkdownMathRuns(t *testing.T) {
	got := renderFixtureDocument(t,
		filepath.Join("testdata", "math_report.sysml"),
		"Optics::OpticsReport")
	for _, want := range []string{
		// Inline LaTeX is trimmed and left unescaped between the dollars.
		`scales as $m \propto D^{2.5}_{\text{eff}}$ and`,
		// A dollar in prose is escaped so it cannot open math.
		`each \$ of budget buys about 1 cm^2`,
		// Neighboring styled runs, refs and links are untouched.
		`*see* [Collecting area of a circular mirror](#overview-mirrorArea) and the [aperture memo](<https://example.com/aperture>)`,
		// The formula is anchored and captioned like a table; blank source
		// lines drop, other lines keep their breaks.
		"<a id=\"overview-mirrorArea\"></a>\n\n<!-- caption -->\n*Collecting area of a circular mirror*\n\n$$\nA = \\pi \\left(\\frac{D}{2}\\right)^2\n= \\frac{\\pi D^2}{4}\n$$",
		// Escaped dollars stay, bare ones gain an escape.
		"$$\n\\text{cost} = 10^6\\,\\$ \\times D^{2.5} + \\$\n$$",
		// Query-backed math runs.
		`- Rayleigh criterion $\theta = 1.22\,\frac{\lambda}{D}$`,
		`- f-number $N = \frac{f}{D}$`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendering does not contain %q\n%s", want, got)
		}
	}
	for _, unwanted := range []string{`\\frac`, `\_`, `\\,`, `^{2.5}\_`} {
		if strings.Contains(got, unwanted) {
			t.Errorf("rendering contains %q, math must not be escaped as prose\n%s", unwanted, got)
		}
	}
}

// TestHTMLMathReportGolden locks the standalone HTML of the math report
// against a committed golden file.
func TestHTMLMathReportGolden(t *testing.T) {
	got := renderFixtureHTML(t, filepath.Join("testdata", "math_report.sysml"),
		"Optics::OpticsReport", HTMLOptions{Fragment: true})
	checkGolden(t, got, filepath.Join("testdata", "math_report.fragment.golden.html"))
}

// TestHTMLMathMarkup checks math is marked up for a script to typeset, never
// as prose: inline LaTeX in a math span with inline delimiters, a formula as
// a figure with display delimiters and a caption, every value escaped.
func TestHTMLMathMarkup(t *testing.T) {
	got := renderFixtureHTML(t, filepath.Join("testdata", "math_report.sysml"),
		"Optics::OpticsReport", HTMLOptions{})
	for _, want := range []string{
		`scales as <span class="sysml-math">\(m \propto D^{2.5}_{\text{eff}}\)</span> and each $ of budget`,
		`<figure class="sysml-formula" id="overview-mirrorArea" data-content="formula" data-name="mirrorArea">`,
		"<div class=\"sysml-math\">\\[A = \\pi \\left(\\frac{D}{2}\\right)^2\n\n  = \\frac{\\pi D^2}{4}\\]</div>",
		`<figcaption class="sysml-caption">Collecting area of a circular mirror</figcaption>`,
		`<a class="sysml-ref" href="#overview-mirrorArea">Collecting area of a circular mirror</a>`,
		`<figure class="sysml-formula" data-content="formula" data-name="dollars">`,
		`<div class="sysml-math">\[\text{cost} = 10^6\,\$ \times D^{2.5} + $\]</div>`,
		`<li class="sysml-item" data-element="Optics::telescope::focalRatio" data-element-kind="partUsage">f-number <span class="sysml-math">\(N = \frac{f}{D}\)</span></li>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendering does not contain %q\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"<script", "<code>\\(", "\\\\(", "\\\\frac"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("rendering contains %q\n%s", unwanted, got)
		}
	}
	// Markup in LaTeX is escaped, not emitted.
	if got := inlineMathHTML(" a < b & c > d\n</span> "); got != `\(a &lt; b &amp; c &gt; d &lt;/span&gt;\)` {
		t.Errorf("inlineMathHTML = %q", got)
	}
	if got := displayMathHTML("\r\n<x>\r\n\\\\\ny\n"); got != "\\[&lt;x&gt;\n\\\\\ny\\]" {
		t.Errorf("displayMathHTML = %q", got)
	}
}

// TestHTMLMathScript checks a page asked to load a math script carries its
// configuration and the one script element after the document, that a
// fragment and a default page load none, and that the configuration confines
// typesetting to math spans.
func TestHTMLMathScript(t *testing.T) {
	path := filepath.Join("testdata", "math_report.sysml")
	url := `https://cdn.example/mathjax.js?a=1&b="2"`
	got := renderFixtureHTML(t, path, "Optics::OpticsReport", HTMLOptions{MathScript: url})
	script := `<script src="https://cdn.example/mathjax.js?a=1&amp;b=&#34;2&#34;"></script>`
	if strings.Count(got, "<script") != 2 || !strings.Contains(got, mathConfig+script) {
		t.Errorf("page lacks the configuration and script %s:\n%s", script, got)
	}
	if strings.Index(got, "</article>") > strings.Index(got, script) || !strings.HasSuffix(got, script+"\n</body>\n</html>\n") {
		t.Errorf("script must follow the document, before </body>:\n%s", got)
	}
	for _, want := range []string{`processHtmlClass: "sysml-math"`, `ignoreHtmlClass: "sysml-document"`, `inlineMath: [["\\(", "\\)"]]`, `displayMath: [["\\[", "\\]"]]`} {
		if !strings.Contains(mathConfig, want) {
			t.Errorf("configuration lacks %s:\n%s", want, mathConfig)
		}
	}
	both := renderFixtureHTML(t, path, "Optics::OpticsReport", HTMLOptions{MermaidScript: "https://cdn.example/mermaid.js", MathScript: url})
	if strings.Count(both, "<script") != 3 || strings.Index(both, "mermaid.js") > strings.Index(both, "mathjax.js") {
		t.Errorf("page must load Mermaid, then the math configuration and script:\n%s", both)
	}
	for name, opts := range map[string]HTMLOptions{
		"default":  {},
		"fragment": {Fragment: true, MathScript: url},
	} {
		if out := renderFixtureHTML(t, path, "Optics::OpticsReport", opts); strings.Contains(out, "<script") {
			t.Errorf("%s page loads a script:\n%s", name, out)
		}
	}
}

// TestMarkdownMathSpan locks the inline math helper: trimming, newline
// folding, dollar escaping and protection of the closing delimiter.
func TestMarkdownMathSpan(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"E = mc^2", "$E = mc^2$"},
		{"  x_1 + x_2  ", "$x_1 + x_2$"},
		{"a\nb\r\nc", "$a b c$"},
		{`\$ 5 + $5`, `$\$ 5 + \$5$`},
		{`\\`, `$\\$`},
		{`x\`, `$x\\$`},
		{`\\\`, `$\\\\$`},
	} {
		if got := mathSpan(c.in); got != c.want {
			t.Errorf("mathSpan(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestMarkdownDisplayMath locks the display math body: lines trimmed, blank
// lines dropped, dollars escaped.
func TestMarkdownDisplayMath(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"E = mc^2", "E = mc^2"},
		{"a \\\\\n\n  b\r\n", "a \\\\\nb"},
		{"$$", `\$\$`},
		{`\$`, `\$`},
	} {
		if got := displayMath(c.in); got != c.want {
			t.Errorf("displayMath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
