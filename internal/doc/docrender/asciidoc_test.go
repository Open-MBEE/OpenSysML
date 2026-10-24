package docrender

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// renderFixtureAsciiDoc evaluates a fixture document and renders it as AsciiDoc.
func renderFixtureAsciiDoc(t *testing.T, path, name string, opts AsciiDocOptions) string {
	t.Helper()
	adoc, err := AsciiDoc(fixtureDocument(t, filepath.Join("testdata", path), name), opts)
	if err != nil {
		t.Fatalf("render document %s: %v", name, err)
	}
	return adoc
}

// TestAsciiDocGoldens locks the AsciiDoc rendering of the fixtures the Markdown
// goldens cover: sections, tables with widths and groups, lists, definitions,
// inline runs, references with anchors, formulas, images and diagrams.
func TestAsciiDocGoldens(t *testing.T) {
	cases := []struct {
		fixture, name, golden string
		opts                  AsciiDocOptions
	}{
		{"telescope_report.sysml", "Observatory::MassReport", "telescope_report.golden.adoc", AsciiDocOptions{}},
		{"quantity_report.sysml", "Launcher::MassReport", "quantity_report.golden.adoc", AsciiDocOptions{}},
		{"derived_report.sysml", "Derived::MassReport", "derived_report.golden.adoc", AsciiDocOptions{}},
		{"math_report.sysml", "Optics::OpticsReport", "math_report.golden.adoc", AsciiDocOptions{}},
		{"image_report.sysml", "Pictures::ImageReport", "image_report.golden.adoc", AsciiDocOptions{}},
		{"referenced_report.sysml", "Referenced::ReferencedReport", "referenced_report.golden.adoc", AsciiDocOptions{NumberFigures: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := renderFixtureAsciiDoc(t, c.fixture, c.name, c.opts)
			checkGolden(t, got, filepath.Join("testdata", c.golden))
		})
	}
}

// TestAsciiDocLinkedDocumentsGolden locks a linked set: xref macros into the
// other file, block anchors on the referenced blocks, in-document references.
func TestAsciiDocLinkedDocumentsGolden(t *testing.T) {
	documents := fixtureDocumentSet(t, filepath.Join("testdata", "linked_reports.sysml"),
		[]string{"Observatory::SystemReport", "Observatory::Mass Appendix"})
	for _, document := range documents {
		got, err := AsciiDoc(document, AsciiDocOptions{})
		if err != nil {
			t.Fatalf("render document %s: %v", document.Name(), err)
		}
		checkGolden(t, got, filepath.Join("testdata", "linked", AsciiDocFileName(document.Name())))
	}
}

// TestAsciiDocStructure checks the shape of the worked example without
// reading the golden: header, section titles, block titles, tables, lists.
func TestAsciiDocStructure(t *testing.T) {
	got := renderFixtureAsciiDoc(t, "telescope_report.sysml", "Observatory::MassReport", AsciiDocOptions{})
	for _, want := range []string{
		"= ", "\n== ", "\n|===\n", `[options="header"]`, "\n* ", "\n.", "[source,mermaid]\n----\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("AsciiDoc lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, ":stem:") {
		t.Errorf("a document without math declares stem:\n%s", got)
	}
	math := renderFixtureAsciiDoc(t, "math_report.sysml", "Optics::OpticsReport", AsciiDocOptions{})
	for _, want := range []string{"= Optics Design Relations\n:stem: latexmath\n\n", "stem:[", "[stem]\n++++\n"} {
		if !strings.Contains(math, want) {
			t.Errorf("math AsciiDoc lacks %q:\n%s", want, math)
		}
	}
}

// TestAsciiDocEscaping checks model text cannot open AsciiDoc structure: the
// adversarial strings the Markdown tests use, in prose, cells and at block start.
func TestAsciiDocEscaping(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"a|b", "a|b"},
		{"*em* _u_ #h `c`", "&#42;em&#42; &#95;u&#95; &#35;h &#96;c&#96;"},
		{"<script>&amp;", "&lt;script&gt;&amp;amp;"},
		{`back\slash`, "back&#92;slash"},
		{"[link]", "&#91;link&#93;"},
		{"$5 and $x$", "$5 and $x$"},
		{"a^2 ~x~ {attr} +pass+", "a&#94;2 &#126;x&#126; &#123;attr} &#43;pass&#43;"},
		{"term:: desc; x;; y", "term:&#58; desc; x;&#59; y"},
		{"https://example.com", "https&#58;//example.com"},
		{"(C) (R) (TM) -- ... -> ((index))", "(&#67;) (&#82;) (&#84;M) -&#45; .&#46;. -&gt; (&#40;index))"},
		{"two\nlines", "two lines"},
		{"a\r\nb\rc", "a b c"},
	} {
		if got := asciiDocInline(c.in); got != c.want {
			t.Errorf("asciiDocInline(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := asciiDocTableCell("two\nlines|cell"); got != "two +\nlines\\|cell" {
		t.Errorf("asciiDocTableCell = %q", got)
	}
	if got := asciiDocTableCell("a\r\nb\rc"); got != "a +\nb +\nc" {
		t.Errorf("asciiDocTableCell carriage returns = %q", got)
	}
	for _, c := range []struct{ in, want string }{
		{".title", "{empty}.title"},
		{"* bullet", "{empty}* bullet"},
		{"- bullet", "{empty}- bullet"},
		{"----", "{empty}----"},
		{"== heading", "{empty}== heading"},
		{"12. ordered", "{empty}12. ordered"},
		{"a. ordered", "{empty}a. ordered"},
		{"iv) ordered", "{empty}iv) ordered"},
		{"// comment", "{empty}// comment"},
		{":attr: value", "{empty}:attr: value"},
		{"|===", "{empty}|==="},
		{"'''", "{empty}'''"},
		{"<<<", "{empty}<<<"},
		{"NOTE: careful", "{empty}NOTE: careful"},
		{"    indented", "indented"},
		{"\t- bullet", "{empty}- bullet"},
		{"42 plain", "42 plain"},
		{"-5 kg", "-5 kg"},
		{"_x_ leads", "_x_ leads"},
		{"*x* leads", "*x* leads"},
		{"<<a,b>> leads", "<<a,b>> leads"},
	} {
		if got := asciiDocBlockStart(c.in); got != c.want {
			t.Errorf("asciiDocBlockStart(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := asciiDocMacroText("a]b\nc"); got != `a\]b c` {
		t.Errorf("asciiDocMacroText = %q", got)
	}
	if got := asciiDocTarget("a b[c].png"); got != "a%20b%5Bc%5D.png" {
		t.Errorf("asciiDocTarget = %q", got)
	}
	if got := asciiDocAttribute(`alt, "quoted"`); got != "alt&#44; &quot;quoted&quot;" {
		t.Errorf("asciiDocAttribute = %q", got)
	}
	if got := asciiDocDelimitedBlock('+', "a\n++++\nb"); got != "+++++\na\n++++\nb\n+++++" {
		t.Errorf("asciiDocDelimitedBlock = %q", got)
	}
}

// TestAsciiDocNilDocument checks the typed error for a missing document.
func TestAsciiDocNilDocument(t *testing.T) {
	_, err := AsciiDoc(nil, AsciiDocOptions{})
	var typed *Error
	if !errors.As(err, &typed) || typed.Kind != ErrorNilDocument {
		t.Fatalf("AsciiDoc(nil) error = %v, want %s", err, ErrorNilDocument)
	}
}

// TestAsciiDocFileName checks the set's file names take the .adoc extension
// over the shared stem.
func TestAsciiDocFileName(t *testing.T) {
	if got := AsciiDocFileName("Observatory::Mass Appendix"); got != "Observatory-Mass.20Appendix.adoc" {
		t.Errorf("AsciiDocFileName = %q", got)
	}
}
