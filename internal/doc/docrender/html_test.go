package docrender

import (
	"errors"
	"html"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

// htmlClassVocabulary is the documented class surface: a theme may rely on
// these names, and the backend emits no others.
var htmlClassVocabulary = map[string]bool{
	"sysml-document": true, "sysml-title": true, "sysml-title-page": true,
	"sysml-toc": true, "sysml-toc-title": true,
	"sysml-section": true, "sysml-section-number": true, "sysml-paragraph": true,
	"sysml-table": true, "sysml-group": true, "sysml-group-heading": true,
	"sysml-group-column": true, "sysml-group-key": true,
	"sysml-row": true, "sysml-cell": true, "sysml-value": true, "sysml-element": true, "sysml-object": true,
	"sysml-separator": true, "sysml-list": true, "sysml-item": true,
	"sysml-definitions": true, "sysml-entry": true, "sysml-term": true, "sysml-description": true,
	"sysml-diagram": true, "sysml-caption": true, "sysml-link": true, "sysml-ref": true,
	"sysml-formula": true, "sysml-math": true,
	"mermaid": true, "dot": true, "plantuml": true,
}

// renderFixtureHTML evaluates a fixture document and renders it as HTML.
func renderFixtureHTML(t *testing.T, path, name string, opts HTMLOptions) string {
	t.Helper()
	out, err := HTML(fixtureDocument(t, path, name), opts)
	if err != nil {
		t.Fatalf("render document %s as HTML: %v", name, err)
	}
	return out
}

func checkGolden(t *testing.T, got, golden string) {
	t.Helper()
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
		t.Errorf("rendered HTML differs from %s (run with -update after intentional changes)\ngot:\n%s", golden, got)
	}
}

// TestHTMLTelescopeReportGolden locks the standalone rendering of the
// telescope report: page shell, layered stylesheet, sections, tables, lists
// and escaped content.
func TestHTMLTelescopeReportGolden(t *testing.T) {
	got := renderFixtureHTML(t, filepath.Join("testdata", "telescope_report.sysml"),
		"Observatory::MassReport", HTMLOptions{})
	checkGolden(t, got, filepath.Join("testdata", "telescope_report.golden.html"))
}

// TestHTMLTelescopeReportFragmentGolden locks the fragment rendering with a
// title page, a table of contents and numbered sections.
func TestHTMLTelescopeReportFragmentGolden(t *testing.T) {
	got := renderFixtureHTML(t, filepath.Join("testdata", "telescope_report.sysml"),
		"Observatory::MassReport",
		HTMLOptions{Fragment: true, TitlePage: true, TOC: true, NumberSections: true})
	checkGolden(t, got, filepath.Join("testdata", "telescope_report.fragment.golden.html"))
}

// TestHTMLMermaidScript checks a page asked to load Mermaid carries the script
// element after the document, configured to draw the page's charts whatever
// their size, a fragment none, and a default page none.
func TestHTMLMermaidScript(t *testing.T) {
	path := filepath.Join("testdata", "telescope_report.sysml")
	url := `https://cdn.example/mermaid.js?a=1&b="2"`
	got := renderFixtureHTML(t, path, "Observatory::MassReport", HTMLOptions{MermaidScript: url})
	script := `<script src="https://cdn.example/mermaid.js?a=1&amp;b=&#34;2&#34;"></script>`
	if strings.Count(got, "<script") != 2 || !strings.Contains(got, script) {
		t.Errorf("page lacks the script element %s and its configuration:\n%s", script, got)
	}
	config := regexp.MustCompile(`<script>mermaid\.initialize\(\{maxTextSize: (\d+), maxEdges: (\d+)\}\);</script>`)
	limits := config.FindStringSubmatch(got)
	if limits == nil {
		t.Fatalf("page lacks the Mermaid configuration:\n%s", got)
	}
	if strings.Index(got, "</article>") > strings.Index(got, script) || !strings.HasSuffix(got, script+"\n"+limits[0]+"\n</body>\n</html>\n") {
		t.Errorf("script and configuration must follow the document, before </body>:\n%s", got)
	}
	if !strings.Contains(got, `<pre class="mermaid">`) {
		t.Errorf("diagram source must stay for the script to draw:\n%s", got)
	}
	textSize, _ := strconv.Atoi(limits[1])
	edges, _ := strconv.Atoi(limits[2])
	arrows := regexp.MustCompile(`(?m)^\s*\S+ (-->|---|-\.->)`)
	for _, chart := range regexp.MustCompile(`(?s)<pre class="mermaid">(.*?)</pre>`).FindAllStringSubmatch(got, -1) {
		source := html.UnescapeString(chart[1])
		drawn := len(arrows.FindAllString(source, -1))
		if drawn == 0 || len(source) >= textSize || drawn >= edges {
			t.Errorf("limits %s, %s do not cover a chart of %d bytes and %d edges", limits[1], limits[2], len(source), drawn)
		}
	}
	if textSize > 50000 || edges > 500 {
		t.Errorf("limits %s, %s are not sized to the fixture's small charts", limits[1], limits[2])
	}
	if out := renderFixtureHTML(t, path, "Observatory::MassReport", HTMLOptions{MermaidScript: url, DiagramForm: view.FormDot}); strings.Contains(out, "mermaid.initialize") {
		t.Errorf("a page drawing no Mermaid chart configures Mermaid:\n%s", out)
	}
	for name, opts := range map[string]HTMLOptions{
		"default":  {},
		"fragment": {Fragment: true, MermaidScript: url},
	} {
		if out := renderFixtureHTML(t, path, "Observatory::MassReport", opts); strings.Contains(out, "<script") {
			t.Errorf("%s page loads a script:\n%s", name, out)
		}
	}
}

// TestHTMLSemanticStructure checks the semantic skeleton and the model facts
// carried on it: the document element, nested sections at valid heading
// levels, typed rows and cells, and a diagram figure.
func TestHTMLSemanticStructure(t *testing.T) {
	got := renderFixtureHTML(t, filepath.Join("testdata", "telescope_report.sysml"),
		"Observatory::MassReport", HTMLOptions{})
	for _, want := range []string{
		`<article class="sysml-document" data-document="Observatory::MassReport">`,
		`<h1 class="sysml-title">Telescope Mass Report</h1>`,
		`<section class="sysml-section"`,
		` data-content="section"`,
		`<th scope="col" data-column="mass">mass</th>`,
		`<tr class="sysml-row" data-element="Observatory::telescope::baffle|shroud *tricky*" data-element-kind="partUsage">`,
		`<td class="sysml-cell" data-column="mass" data-value-kind="real"><span class="sysml-value" data-value-kind="real">1.5</span></td>`,
		`<li class="sysml-item" data-element="Observatory::telescope::mount" data-element-kind="partUsage">`,
		`<pre class="mermaid">`,
		`<span class="sysml-value sysml-element" data-value-kind="element" data-element="Observatory::Assembly *frame*" data-element-kind="partDef">`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendering does not contain %q\n%s", want, got)
		}
	}
	if strings.Contains(got, "<h7") {
		t.Error("rendering writes a heading level HTML has not")
	}
	// Sections nest as elements, so every one that opens is closed.
	if open, closed := strings.Count(got, "<section "), strings.Count(got, "</section>"); open != closed {
		t.Errorf("%d sections opened, %d closed", open, closed)
	}
}

// TestHTMLQuantityCells checks that a quantity cell keeps its unit in the text
// and carries magnitude and unit apart as data attributes.
func TestHTMLQuantityCells(t *testing.T) {
	got := renderFixtureHTML(t, filepath.Join("testdata", "quantity_report.sysml"),
		"Launcher::MassReport", HTMLOptions{})
	for _, want := range []string{
		`<td class="sysml-cell" data-column="mass" data-value-kind="quantity"><span class="sysml-value" data-value-kind="quantity" data-magnitude="2290000" data-unit="kg">2290000 [kg]</span></td>`,
		`<td class="sysml-cell" data-column="mass" data-value-kind="quantity"><span class="sysml-value" data-value-kind="quantity" data-magnitude="500000" data-unit="g">500000 [g]</span></td>`,
		`<td class="sysml-cell" data-column="tonnes" data-value-kind="quantity"><span class="sysml-value" data-value-kind="quantity" data-magnitude="2290" data-unit="kg">2290 [kg]</span></td>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendering does not contain %q\n%s", want, got)
		}
	}
	if strings.Count(got, `data-column="mass" data-value-kind="quantity"`) != 4 {
		t.Errorf("rendering does not carry four mass cells\n%s", got)
	}
}

// TestHTMLDerivedQuantityCells checks that a quantity derived from other
// features renders as a quantity cell — unit in the text, magnitude and unit
// apart as data attributes — and reaches list items and definitions.
func TestHTMLDerivedQuantityCells(t *testing.T) {
	got := renderFixtureHTML(t, filepath.Join("testdata", "derived_report.sysml"),
		"Derived::MassReport", HTMLOptions{})
	for _, want := range []string{
		`<td class="sysml-cell" data-column="mass" data-value-kind="quantity"><span class="sysml-value" data-value-kind="quantity" data-magnitude="2290000" data-unit="kg">2290000 [kg]</span></td>`,
		`<td class="sysml-cell" data-column="mass" data-value-kind="quantity"><span class="sysml-value" data-value-kind="quantity" data-magnitude="2280000" data-unit="kg">2280000 [kg]</span></td>`,
		`<td class="sysml-cell" data-column="engines" data-value-kind="integer"><span class="sysml-value" data-value-kind="integer">3</span></td>`,
		`<td class="sysml-cell" data-column="class" data-value-kind="string"><span class="sysml-value" data-value-kind="string">light</span></td>`,
		`<td class="sysml-cell" data-column="perEngine" data-value-kind="quantity"><span class="sysml-value" data-value-kind="quantity" data-magnitude="458000" data-unit="kg">458000 [kg]</span></td>`,
		`<li class="sysml-item" data-element="Derived::rocket::s1" data-element-kind="partUsage">s1 2290000 [kg]</li>`,
		`<dt class="sysml-term">rocket</dt>`,
		`<dd class="sysml-description">4689000 [kg]</dd>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendering does not contain %q\n%s", want, got)
		}
	}
	if strings.Count(got, `data-column="mass" data-value-kind="quantity"`) != 3 {
		t.Errorf("rendering does not carry three mass cells\n%s", got)
	}
	if strings.Contains(got, `data-element="Derived::rocket::s2" data-element-kind="partUsage">s2`) {
		t.Errorf("list holds s2, whose derived mass is not above the threshold\n%s", got)
	}
}

// TestHTMLDiagramForm writes every graph-shaped diagram as DOT when the render
// asks for it, leaves the table a table, and rejects a form that is not one.
func TestHTMLDiagramForm(t *testing.T) {
	path := filepath.Join("testdata", "telescope_report.sysml")
	got := renderFixtureHTML(t, path, "Observatory::MassReport", HTMLOptions{DiagramForm: view.FormDot})
	for _, want := range []string{
		`<pre class="dot">// view: Observatory::interconnectView` + "\n// kind: interconnection\n",
		"// layout: dot\ndigraph &#34;Observatory::interconnectView&#34; {\n",
		`<pre class="dot">// kind: state` + "\n",
		"// layout: dot\ndigraph {\n  graph [fontname=&#34;Helvetica&#34;, rankdir=LR];\n",
		`<table class="sysml-table"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendering does not contain %q\n%s", want, got)
		}
	}
	if strings.Contains(got, `class="mermaid"`) {
		t.Errorf("a diagram is still Mermaid when DOT is asked for:\n%s", got)
	}
	plantuml := renderFixtureHTML(t, path, "Observatory::MassReport", HTMLOptions{DiagramForm: view.FormPlantUML})
	for _, want := range []string{
		`<pre class="plantuml">@startuml` + "\n&#39; Observatory::interconnectView — interconnection rendering",
		`<pre class="plantuml">@startuml` + "\n&#39; state rendering",
		"left to right direction\n",
		`<table class="sysml-table"`,
	} {
		if !strings.Contains(plantuml, want) {
			t.Errorf("PlantUML rendering does not contain %q\n%s", want, plantuml)
		}
	}
	if strings.Contains(plantuml, `class="mermaid"`) || strings.Contains(plantuml, `class="dot"`) {
		t.Errorf("a diagram is in another form when PlantUML is asked for:\n%s", plantuml)
	}
	if !strings.Contains(renderFixtureHTML(t, path, "Observatory::MassReport", HTMLOptions{}), `<pre class="mermaid">`) {
		t.Error("the default diagram form is not Mermaid")
	}
	_, err := HTML(fixtureDocument(t, path, "Observatory::MassReport"), HTMLOptions{DiagramForm: "svg"})
	var typed *Error
	if !errors.As(err, &typed) || typed.Kind != ErrorUnknownForm || typed.DiagramForm != "svg" {
		t.Fatalf("error = %v, want %s for svg", err, ErrorUnknownForm)
	}
}

// TestHTMLNoInlineStylesOrUnknownClasses checks the override contract on the
// markup: nothing carries a style attribute, and every class is one the
// documented vocabulary names.
func TestHTMLNoInlineStylesOrUnknownClasses(t *testing.T) {
	for _, c := range []struct {
		fixture, document string
		opts              HTMLOptions
	}{
		{"telescope_report.sysml", "Observatory::MassReport", HTMLOptions{}},
		{"telescope_report.sysml", "Observatory::MassReport", HTMLOptions{Fragment: true, TitlePage: true, TOC: true, NumberSections: true}},
		{"telescope_report.sysml", "Observatory::MassReport", HTMLOptions{DiagramForm: view.FormDot}},
		{"telescope_report.sysml", "Observatory::MassReport", HTMLOptions{DiagramForm: view.FormPlantUML}},
		{"math_report.sysml", "Optics::OpticsReport", HTMLOptions{}},
	} {
		got := renderFixtureHTML(t, filepath.Join("testdata", c.fixture), c.document, c.opts)
		if strings.Contains(got, "style=\"") {
			t.Error("rendering carries an inline style attribute, which reader CSS cannot override")
		}
		for _, match := range regexp.MustCompile(`class="([^"]*)"`).FindAllStringSubmatch(got, -1) {
			for _, class := range strings.Fields(match[1]) {
				if !htmlClassVocabulary[class] {
					t.Errorf("rendering emits undocumented class %q", class)
				}
			}
		}
	}
}

// TestHTMLIdentifiersAreAnchorsOnly checks every in-document link resolves to
// an identifier the document declares, and that no two nodes share one.
func TestHTMLIdentifiersAreAnchorsOnly(t *testing.T) {
	got := renderFixtureHTML(t, filepath.Join("testdata", "telescope_report.sysml"),
		"Observatory::MassReport", HTMLOptions{TOC: true})
	ids := map[string]bool{}
	for _, match := range regexp.MustCompile(`\sid="([^"]*)"`).FindAllStringSubmatch(got, -1) {
		if ids[match[1]] {
			t.Errorf("identifier %q is declared twice", match[1])
		}
		ids[match[1]] = true
	}
	for _, match := range regexp.MustCompile(`href="#([^"]*)"`).FindAllStringSubmatch(got, -1) {
		if !ids[match[1]] {
			t.Errorf("link to #%s resolves to nothing; declared: %v", match[1], ids)
		}
	}
}

// TestHTMLDefaultStylesheetIsOverridable checks the cascade contract of the
// default stylesheet: it is declared as a layer before use, every rule sits
// inside it, and every value it sets comes from a --sysml-* token.
func TestHTMLDefaultStylesheetIsOverridable(t *testing.T) {
	css := DefaultStylesheet()
	declaration := strings.Index(css, "@layer opensysml;")
	block := strings.Index(css, "@layer opensysml {")
	if declaration < 0 || block < 0 || declaration > block {
		t.Fatalf("default stylesheet must declare @layer opensysml before using it:\n%s", css)
	}
	if strings.Count(css, "@layer") != 2 {
		t.Errorf("default stylesheet writes more than the one layer:\n%s", css)
	}
	literal := regexp.MustCompile(`(#[0-9a-fA-F]{3}|[0-9.]+(px|rem|em|ch|vh|vw|pt|%)|["'])`)
	for _, line := range strings.Split(css[block:], "\n") {
		text := strings.TrimSpace(line)
		property, value, ok := strings.Cut(text, ":")
		if !ok || strings.HasPrefix(strings.TrimSpace(property), "--sysml-") {
			continue
		}
		if literal.MatchString(value) {
			t.Errorf("declaration %q hardcodes a value; take it from a --sysml-* token", text)
		}
	}
}

// TestHTMLThemes checks every bundled theme is one block of the opensysml
// layer, scoped to the document, layered after the default sheet in a page,
// and that a name that is no theme is refused.
func TestHTMLThemes(t *testing.T) {
	names := Themes()
	if want := []string{"default", "acm", "ieee", "modern", "nasa", "print", "report"}; !slices.Equal(names, want) {
		t.Fatalf("Themes() = %v, want %v", names, want)
	}
	plain, err := ThemeStylesheet("")
	if err != nil || plain != DefaultStylesheet() {
		t.Fatalf("an empty theme is the default sheet; got err %v", err)
	}
	if named, _ := ThemeStylesheet(DefaultTheme); named != plain {
		t.Error("the default theme, named, is the default sheet")
	}
	for _, name := range names[1:] {
		css, err := ThemeStylesheet(name)
		if err != nil {
			t.Fatalf("theme %s: %v", name, err)
		}
		if !strings.HasPrefix(css, DefaultStylesheet()) {
			t.Errorf("theme %s does not start from the default sheet", name)
		}
		overrides := css[len(DefaultStylesheet()):]
		if strings.Count(overrides, "@layer opensysml {") != 1 || strings.Contains(overrides, "@layer opensysml;") {
			t.Errorf("theme %s must be exactly one block of the opensysml layer:\n%s", name, overrides)
		}
		if strings.Contains(strings.ToLower(overrides), "</style") {
			t.Errorf("theme %s would close the style element it is inlined in", name)
		}
		// Selectors are the lines ending a rule opener or a selector list entry.
		for _, line := range strings.Split(overrides[strings.Index(overrides, "@layer opensysml {"):], "\n") {
			sel := strings.TrimSpace(line)
			if !strings.HasSuffix(sel, "{") && !strings.HasSuffix(sel, ",") || strings.HasPrefix(sel, "@") {
				continue
			}
			if !strings.HasPrefix(sel, ".sysml-document") {
				t.Errorf("theme %s selector %q is not scoped to .sysml-document", name, sel)
			}
		}
		got := renderFixtureHTML(t, filepath.Join("testdata", "telescope_report.sysml"),
			"Observatory::MassReport", HTMLOptions{
				Theme:       name,
				Stylesheets: []Stylesheet{{Content: ".sysml-document { color: rebeccapurple; }"}},
			})
		base := strings.Index(got, "@layer opensysml {")
		theme := strings.Index(got, "/* "+name+":")
		supplied := strings.Index(got, "rebeccapurple")
		if base < 0 || theme < base || supplied < theme {
			t.Errorf("theme %s: default, theme and supplied CSS must follow in that order:\n%s", name, got)
		}
		if strings.Count(got, "<style>") != 2 {
			t.Errorf("theme %s: default and theme share one style element, supplied CSS has its own:\n%s", name, got)
		}
	}
	for _, bad := range []string{"fancy", "../document", "report.css", "report.print", `themes\report`} {
		_, err := ThemeStylesheet(bad)
		var rendering *Error
		if !errors.As(err, &rendering) || rendering.Kind != ErrorUnknownTheme || rendering.Actual != bad {
			t.Errorf("ThemeStylesheet(%q) = %v, want an unknown-theme error", bad, err)
		}
		if err != nil && !strings.Contains(err.Error(), "default, acm, ieee, modern, nasa, print, report") {
			t.Errorf("ThemeStylesheet(%q) error does not list the themes: %v", bad, err)
		}
	}
	doc := fixtureDocument(t, filepath.Join("testdata", "telescope_report.sysml"), "Observatory::MassReport")
	if _, err := HTML(doc, HTMLOptions{Theme: "fancy"}); err == nil {
		t.Error("HTML accepted a theme that does not exist")
	}
	if _, err := HTML(doc, HTMLOptions{Theme: "fancy", NoDefaultStylesheet: true}); err != nil {
		t.Errorf("HTML checked a theme it was told to leave out: %v", err)
	}
	// print spells out web links however the scheme is spelled.
	printCSS, err := ThemeStylesheet("print")
	if err != nil {
		t.Fatal(err)
	}
	for _, sel := range []string{`[href^="http:" i]::after`, `[href^="https:" i]::after`, `[href^="//"]::after`} {
		if !strings.Contains(printCSS, ".sysml-link"+sel) {
			t.Errorf("print theme lacks the link selector %s", sel)
		}
	}
}

// TestHTMLThemePrintCompanions checks a theme's print companion is no theme of
// its own but comes with its theme: one block of the opensysml-print-theme
// layer, writing page geometry, page-margin boxes and document tokens only,
// and that the default and a theme without one have none.
func TestHTMLThemePrintCompanions(t *testing.T) {
	entries, err := fs.ReadDir(themeFS, "themes")
	if err != nil {
		t.Fatal(err)
	}
	var companions []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".print.css") {
			companions = append(companions, strings.TrimSuffix(entry.Name(), ".print.css"))
		}
	}
	if want := []string{"acm", "ieee", "nasa", "print", "report"}; !slices.Equal(companions, want) {
		t.Fatalf("print companions = %v, want %v", companions, want)
	}
	for _, name := range Themes() {
		if strings.HasSuffix(name, ".print") {
			t.Errorf("Themes() lists the companion %s as a theme", name)
		}
	}
	for _, name := range []string{"", DefaultTheme, "modern"} {
		if css, err := ThemePrintStylesheet(name); err != nil || css != "" {
			t.Errorf("ThemePrintStylesheet(%q) = %q, %v; want none", name, css, err)
		}
	}
	for _, bad := range []string{"fancy", "report.print", "../document"} {
		_, err := ThemePrintStylesheet(bad)
		var rendering *Error
		if !errors.As(err, &rendering) || rendering.Kind != ErrorUnknownTheme || rendering.Actual != bad {
			t.Errorf("ThemePrintStylesheet(%q) = %v, want an unknown-theme error", bad, err)
		}
	}
	for _, name := range companions {
		css, err := ThemePrintStylesheet(name)
		if err != nil {
			t.Fatalf("companion %s: %v", name, err)
		}
		if !strings.HasPrefix(css, "/* "+name+":") {
			t.Errorf("companion %s does not open with its theme's name:\n%.80s", name, css)
		}
		if strings.Count(css, "@layer opensysml-print-theme {") != 1 || strings.Contains(css, "@layer opensysml {") || strings.Contains(css, "@layer opensysml-print {") {
			t.Errorf("companion %s must be exactly one block of the opensysml-print-theme layer:\n%s", name, css)
		}
		if theme, _ := ThemeStylesheet(name); strings.Contains(theme, css) {
			t.Errorf("companion %s is folded into the theme's screen sheet", name)
		}
		for _, line := range strings.Split(css, "\n") {
			sel := strings.TrimSpace(line)
			if !strings.HasSuffix(sel, "{") && !strings.HasSuffix(sel, ",") || strings.HasPrefix(sel, "@layer") {
				continue
			}
			if !strings.HasPrefix(sel, ":root") && !strings.HasPrefix(sel, ".sysml-document") && !strings.HasPrefix(sel, "@page") && !strings.HasPrefix(sel, "@bottom-") {
				t.Errorf("companion %s selector %q is neither :root, .sysml-document, @page nor a page-margin box", name, sel)
			}
		}
	}
}

// TestHTMLConventionThemes checks the nasa, ieee and acm themes set the
// faces, sizes and black-on-white tokens their conventions call for, on
// screen and in their print companions alike, and that both agree.
func TestHTMLConventionThemes(t *testing.T) {
	times := `"Times New Roman", Times, "Liberation Serif", "Nimbus Roman", serif`
	arial := `Arial, Helvetica, "Liberation Sans", "Nimbus Sans", sans-serif`
	courier := `"Courier New", Courier, "Liberation Mono", "Nimbus Mono PS", monospace`
	libertine := `"Libertinus Serif", "Linux Libertine O", "Linux Libertine", "Times New Roman", "Liberation Serif", serif`
	biolinum := `"Libertinus Sans", "Linux Biolinum O", "Linux Biolinum", Arial, Helvetica, "Liberation Sans", sans-serif`
	cases := []struct {
		theme       string
		body        string
		heading     string
		size        string
		caption     string
		screenOnly  []string
		page        []string
		companion   []string
		pageNumbers string
	}{
		{
			theme: "nasa", body: times, heading: arial, size: "12pt", caption: "12pt",
			screenOnly: []string{"--sysml-measure: 6.5in;", ".sysml-document .sysml-title {\n    font-size: 24pt;"},
			page:       []string{"--sysml-page-size: letter;", "--sysml-page-margin: 1in;", "--sysml-page-number-font-size: 12pt;"},
			companion:  []string{"--sysml-subheading-font-size: 14pt;", "--sysml-subsubheading-font-size: 12pt;", "content: counter(page, lower-roman);", "counter-reset: page 1;"},
		},
		{
			theme: "ieee", body: times, heading: times, size: "10pt", caption: "8pt",
			screenOnly: []string{"--sysml-measure: 7.17in;", "font-variant: small-caps;", "text-align: justify;", "--sysml-paragraph-indent: 1pc;", "--sysml-table-font-size: 8pt;"},
			page:       []string{"--sysml-page-size: letter;", "--sysml-page-margin: 0.67in;", "--sysml-page-number-font-size: 8pt;"},
			companion:  []string{"--sysml-subheading-font-size: 10pt;", "--sysml-wide-table-font-size: 8pt;"},
		},
		{
			theme: "acm", body: libertine, heading: biolinum, size: "10pt", caption: "9pt",
			screenOnly: []string{"--sysml-measure: 6.5in;", "--sysml-paragraph-indent: 10pt;", ".sysml-document .sysml-title {\n    font-size: 17pt;"},
			page:       []string{"--sysml-page-size: letter;", "--sysml-page-margin: 1in;", "--sysml-page-number-font-size: 9pt;"},
			companion:  []string{"--sysml-subheading-font-size: 10pt;", "--sysml-wide-table-font-size: 9pt;"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.theme, func(t *testing.T) {
			full, err := ThemeStylesheet(tc.theme)
			if err != nil {
				t.Fatal(err)
			}
			screen := full[len(DefaultStylesheet()):]
			companion, err := ThemePrintStylesheet(tc.theme)
			if err != nil {
				t.Fatal(err)
			}
			shared := []string{
				"--sysml-font-body: " + tc.body + ";",
				"--sysml-font-heading: " + tc.heading + ";",
				"--sysml-font-mono: " + courier + ";",
				"--sysml-font-size: " + tc.size + ";",
			}
			for _, want := range append(append(append([]string{}, shared...), tc.screenOnly...),
				"--sysml-caption-font-size: "+tc.caption+";",
				"--sysml-text: #000000;", "--sysml-accent: #000000;", "--sysml-rule: #000000;", "--sysml-surface: transparent;",
				"border-bottom: var(--sysml-border-width) solid var(--sysml-rule);", "caption-side: top;",
			) {
				if !strings.Contains(screen, want) {
					t.Errorf("theme %s lacks %q", tc.theme, want)
				}
			}
			for _, want := range append(append(append([]string{}, shared...), tc.page...), tc.companion...) {
				if !strings.Contains(companion, want) {
					t.Errorf("companion %s lacks %q", tc.theme, want)
				}
			}
			for _, stray := range []string{"--sysml-measure", "--sysml-text:", "--sysml-accent:"} {
				if strings.Contains(companion, stray) {
					t.Errorf("companion %s sets %s, which the screen sheet already carries to the page", tc.theme, stray)
				}
			}
			if strings.Contains(screen, "--sysml-page-") {
				t.Errorf("theme %s writes page tokens the screen never reads", tc.theme)
			}
		})
	}
}

// TestHTMLSuppliedStylesheets checks supplied CSS lands after the default
// layer and unlayered, that a URL is linked rather than inlined, and that
// leaving the default out leaves the document unstyled.
func TestHTMLSuppliedStylesheets(t *testing.T) {
	got := renderFixtureHTML(t, filepath.Join("testdata", "telescope_report.sysml"),
		"Observatory::MassReport", HTMLOptions{
			Stylesheets: []Stylesheet{
				{Content: ".sysml-document { color: rebeccapurple; }"},
				{Href: "https://example.test/theme.css"},
			},
		})
	layer := strings.Index(got, "@layer opensysml {")
	supplied := strings.Index(got, "rebeccapurple")
	link := strings.Index(got, `<link rel="stylesheet" href="https://example.test/theme.css">`)
	if layer < 0 || supplied < layer || link < supplied {
		t.Fatalf("supplied stylesheets must follow the default layer in order:\n%s", got)
	}
	if strings.Contains(got[supplied-200:supplied], "@layer") {
		t.Error("supplied CSS is layered; it must stay unlayered to win on cascade origin")
	}
	bare := renderFixtureHTML(t, filepath.Join("testdata", "telescope_report.sysml"),
		"Observatory::MassReport", HTMLOptions{NoDefaultStylesheet: true})
	if strings.Contains(bare, "<style>") {
		t.Error("-html-no-default-css must leave no stylesheet behind")
	}
	fragment := renderFixtureHTML(t, filepath.Join("testdata", "telescope_report.sysml"),
		"Observatory::MassReport", HTMLOptions{Fragment: true})
	if strings.Contains(fragment, "<html") || strings.Contains(fragment, "<style>") {
		t.Error("a fragment carries neither the page shell nor a stylesheet")
	}
	if !strings.HasPrefix(fragment, `<article class="sysml-document"`) {
		t.Errorf("a fragment starts at the document element:\n%s", fragment)
	}
}

// TestHTMLEmptyStylesheet checks a stylesheet the run named but which declares
// nothing is inlined empty, rather than rejected as an unspecified sheet.
func TestHTMLEmptyStylesheet(t *testing.T) {
	document := fixtureDocument(t, filepath.Join("testdata", "telescope_report.sysml"), "Observatory::MassReport")
	got, err := HTML(document, HTMLOptions{Stylesheets: []Stylesheet{InlineStylesheet("")}})
	if err != nil {
		t.Fatalf("an empty stylesheet file is a stylesheet: %v", err)
	}
	if !strings.Contains(got, "@layer opensysml {") {
		t.Errorf("the default stylesheet is still carried:\n%s", got)
	}
}

// TestHTMLStylesheetErrors checks the typed errors for a stylesheet that is
// neither content nor URL, both at once, or would close its style element.
func TestHTMLStylesheetErrors(t *testing.T) {
	document := fixtureDocument(t, filepath.Join("testdata", "telescope_report.sysml"), "Observatory::MassReport")
	for _, c := range []struct {
		sheet Stylesheet
		kind  ErrorKind
	}{
		{Stylesheet{}, ErrorEmptyStylesheet},
		{Stylesheet{Content: "a{}", Href: "theme.css"}, ErrorAmbiguousStylesheet},
		{Stylesheet{Content: "a{}</STYLE><script>alert(1)</script>"}, ErrorUnsafeStylesheet},
	} {
		_, err := HTML(document, HTMLOptions{Stylesheets: []Stylesheet{c.sheet}})
		var typed *Error
		if !errors.As(err, &typed) || typed.Kind != c.kind {
			t.Errorf("HTML(%+v) error = %v, want %s", c.sheet, err, c.kind)
		}
	}
}

// TestHTMLEscaping checks no content can corrupt the structure: markup
// characters, quotes, closing tags and newlines in text, attributes and
// comments.
func TestHTMLEscaping(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{`<b>&plain</b>`, `&lt;b&gt;&amp;plain&lt;/b&gt;`},
		{`quote " apostrophe '`, `quote &#34; apostrophe &#39;`},
		{`</script><script>alert(1)</script>`, `&lt;/script&gt;&lt;script&gt;alert(1)&lt;/script&gt;`},
		{"two\nlines\r\nand\rmore", "two lines and more"},
	} {
		if got := htmlText(c.in); got != c.want {
			t.Errorf("htmlText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := attr("data-name", `a"b`); got != ` data-name="a&#34;b"` {
		t.Errorf("attr = %q", got)
	}
	if got := attr("data-name", ""); got != "" {
		t.Errorf("an empty value writes no attribute, got %q", got)
	}
	if got := htmlComment("closes --> early\nand wraps"); got != "closes - -> early and wraps" {
		t.Errorf("htmlComment = %q", got)
	}
}

// TestHTMLLinkSchemes checks a link is an href only for a scheme a document
// navigates to; a script URL is kept as data instead.
func TestHTMLLinkSchemes(t *testing.T) {
	for _, target := range []string{"https://example.test/a", "mailto:a@example.test", "#anchor", "report.html", "./a:b/c"} {
		if _, ok := navigableURL(target); !ok {
			t.Errorf("navigableURL(%q) = false, want true", target)
		}
	}
	for _, target := range []string{"javascript:alert(1)", "JavaScript:alert(1)", "data:text/html,<script>"} {
		if _, ok := navigableURL(target); ok {
			t.Errorf("navigableURL(%q) = true, want false", target)
		}
	}
	if got, _ := navigableURL("https://example.test/a\nb"); got != "https://example.test/ab" {
		t.Errorf("newlines are stripped from a URL, got %q", got)
	}
}

// TestHTMLDeterministic checks repeated rendering of one document is
// byte-identical.
func TestHTMLDeterministic(t *testing.T) {
	document := fixtureDocument(t, filepath.Join("testdata", "telescope_report.sysml"), "Observatory::MassReport")
	opts := HTMLOptions{TOC: true, NumberSections: true, TitlePage: true}
	first, err := HTML(document, opts)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for i := 0; i < 3; i++ {
		again, err := HTML(document, opts)
		if err != nil {
			t.Fatalf("render again: %v", err)
		}
		if again != first {
			t.Fatal("repeated rendering is not byte-identical")
		}
	}
}

// TestHTMLNilDocument checks the typed error for a missing document.
func TestHTMLNilDocument(t *testing.T) {
	_, err := HTML(nil, HTMLOptions{})
	var typed *Error
	if !errors.As(err, &typed) || typed.Kind != ErrorNilDocument {
		t.Fatalf("HTML(nil) error = %v, want %s", err, ErrorNilDocument)
	}
}

// TestHTMLAnonymousSections checks a section with no name is still addressable:
// every one gets its own identifier, its contents link resolves, and numbering
// follows the document order.
func TestHTMLAnonymousSections(t *testing.T) {
	document := fixtureDocument(t, filepath.Join("testdata", "anonymous_sections.sysml"), "Anonymous::AnonymousReport")
	got, err := HTML(document, HTMLOptions{TOC: true, NumberSections: true})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	ids := regexp.MustCompile(`<section class="sysml-section" id="([^"]*)"`).FindAllStringSubmatch(got, -1)
	if len(ids) != 4 {
		t.Fatalf("sections with identifiers = %d, want 4:\n%s", len(ids), got)
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if id[1] == "" {
			t.Errorf("a section was written without an identifier:\n%s", got)
		}
		if seen[id[1]] {
			t.Errorf("identifier %q is written twice:\n%s", id[1], got)
		}
		seen[id[1]] = true
	}

	for _, entry := range regexp.MustCompile(`<a href="#([^"]*)">`).FindAllStringSubmatch(got, -1) {
		if !seen[entry[1]] {
			t.Errorf("contents links to #%s, which no section carries:\n%s", entry[1], got)
		}
	}
	for _, number := range []string{"1", "1.1", "1.2", "2"} {
		if !strings.Contains(got, `<span class="sysml-section-number">`+number+`</span>`) {
			t.Errorf("section number %s is missing:\n%s", number, got)
		}
	}
}

// TestHTMLAnonymousSectionLeavesReservedAnchor checks an anonymous section
// numbers itself around the anchor a later named section is referenced by,
// rather than taking the identifier that reference resolves to.
func TestHTMLAnonymousSectionLeavesReservedAnchor(t *testing.T) {
	document := fixtureDocument(t, filepath.Join("testdata", "reserved_anchor.sysml"), "Reserved::ReservedAnchorReport")
	got, err := HTML(document, HTMLOptions{TOC: true})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	ids := regexp.MustCompile(`<section class="sysml-section" id="([^"]*)"`).FindAllStringSubmatch(got, -1)
	if len(ids) != 2 {
		t.Fatalf("sections with identifiers = %d, want 2:\n%s", len(ids), got)
	}
	if ids[0][1] == "section" {
		t.Errorf("the anonymous section took the anchor the named section is referenced by:\n%s", got)
	}
	if ids[1][1] != "section" {
		t.Errorf("the named section's anchor = %q, want \"section\":\n%s", ids[1][1], got)
	}
	if !strings.Contains(got, `<a class="sysml-ref" href="#section"`) {
		t.Errorf("the reference does not resolve to the named section:\n%s", got)
	}
}
