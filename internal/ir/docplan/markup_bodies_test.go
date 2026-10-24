package docplan

import (
	"errors"
	"strings"
	"testing"
)

// bodyPlanningError is the typed planning failure err carries.
func bodyPlanningError(t *testing.T, err error) *Error {
	t.Helper()
	var planning *Error
	if !errors.As(err, &planning) {
		t.Fatalf("got %v, want a planning error", err)
	}
	return planning
}

// A Markdown body directly inside a document expands in place, in declaration
// order among the sibling parts, into paragraphs and lists; a second body
// is a second run of blocks.
func TestCompileDocumentMarkdownBody(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part def Report :> Document {
			attribute redefines title = "Report";
			part opening : Paragraph {
				:>> text = "First.";
			}
			rep language "markdown" /* Second *em* and **strong** and `+"`code`"+`.
			 *
			 * - one
			 * - two
			 */
			part middle : Paragraph {
				:>> text = "Fourth.";
			}
			rep language "Markdown" /* 1. a
			 * 2. b
			 */
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	content := plan.Content()
	kinds := make([]string, len(content))
	for i, node := range content {
		kinds[i] = string(node.Kind())
		if node.Style() != "" {
			kinds[i] += ":" + string(node.Style())
		}
	}
	if got, want := strings.Join(kinds, " "), "paragraph paragraph list:bullet paragraph list:number"; got != want {
		t.Fatalf("content kinds %q, want %q", got, want)
	}
	if content[1].Name() != "" || content[2].Name() != "" {
		t.Errorf("markup blocks are named %q %q, want anonymous", content[1].Name(), content[2].Name())
	}
	runs := content[1].Runs()
	want := []struct {
		style RunStyle
		text  string
	}{
		{StylePlain, "Second "}, {StyleEmphasis, "em"}, {StylePlain, " and "}, {StyleStrong, "strong"},
		{StylePlain, " and "}, {StyleCode, "code"}, {StylePlain, "."},
	}
	if len(runs) != len(want) {
		t.Fatalf("runs = %d, want %d: %+v", len(runs), len(want), runs)
	}
	for i, w := range want {
		if runs[i].Kind() != RunSpan || runs[i].Style() != w.style || runs[i].Text() != w.text {
			t.Errorf("run %d = %s %s %q, want span %s %q", i, runs[i].Kind(), runs[i].Style(), runs[i].Text(), w.style, w.text)
		}
	}
	items := content[2].Items()
	if len(items) != 2 || items[0].Runs()[0].Text() != "one" || items[1].Runs()[0].Text() != "two" {
		t.Errorf("bullet items %+v", items)
	}
	if numbered := content[4].Items(); len(numbered) != 2 || numbered[1].Runs()[0].Text() != "b" {
		t.Errorf("numbered items %+v", numbered)
	}
	if content[2].Query() != nil || len(content[2].ColumnRuns()) != 0 {
		t.Error("a static list carries a query or column runs")
	}
}

// Every run and block of a body is placed inside the body text it came from.
func TestCompileMarkdownBodyOrigins(t *testing.T) {
	src := `
		part def Report :> Document {
			attribute redefines title = "Report";
			rep language "markdown" /* Lead *em* [spec](https://example.com) $x^2$
			 *
			 * - item
			 */
		}
	`
	fixture := loadPlanningFixture(t, src)
	plan := fixture.mustCompile(t, "Report")
	content := plan.Content()
	full := "package Observatory {" + planningImports + src + "}"
	paragraph := content[0]
	if o := paragraph.Origin(); !o.Located() || full[o.Span.Offset:o.Span.End()] != "Lead *em* [spec](https://example.com) $x^2$" {
		t.Errorf("paragraph origin covers %q", full[o.Span.Offset:o.Span.End()])
	}
	for _, want := range []struct {
		index int
		text  string
	}{
		{0, "Lead "}, {1, "em"}, {3, "[spec](https://example.com)"}, {5, "$x^2$"},
	} {
		o := paragraph.Runs()[want.index].Origin()
		if !o.Located() || o.Doc != fixtureDoc || full[o.Span.Offset:o.Span.End()] != want.text {
			t.Errorf("run %d origin covers %q, want %q", want.index, full[o.Span.Offset:o.Span.End()], want.text)
		}
	}
	if o := content[1].Items()[0].Origin(); full[o.Span.Offset:o.Span.End()] != "item" {
		t.Errorf("item origin covers %q", full[o.Span.Offset:o.Span.End()])
	}
	if o := content[1].Origin(); full[o.Span.Offset:o.Span.End()] != "item" {
		t.Errorf("list origin covers %q", full[o.Span.Offset:o.Span.End()])
	}
}

// Links, autolinks, mathematics and references map onto the runs the
// equivalent Span, Link and Ref parts would plan.
func TestCompileMarkdownBodyLinksAndRefs(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part def Telescope { part mount; }
		part telescope : Telescope;
		part def Report :> Document {
			attribute redefines title = "Report";
			rep language "markdown" /* See [the spec](https://example.com/spec), <https://example.com>,
			 * [details](sysml:details), [](sysml:details), [mount](sysml:telescope.mount),
			 * [](sysml:Telescope.metadata) and $m \propto D^2$.
			 */
			part viaPart : Paragraph {
				part r : Ref { ref redefines target = telescope.mount; }
			}
			part details : Section {
				attribute redefines title = "Details";
				part body : Paragraph { :>> text = "body"; }
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	runs := plan.Content()[0].Runs()
	byKind := map[RunKind][]Run{}
	for _, run := range runs {
		byKind[run.Kind()] = append(byKind[run.Kind()], run)
	}
	links := byKind[RunLink]
	if len(links) != 2 || links[0].Text() != "the spec" || links[0].Target() != "https://example.com/spec" ||
		links[1].Text() != "https://example.com" || links[1].Target() != "https://example.com" {
		t.Errorf("links %+v", links)
	}
	refs := byKind[RunRef]
	if len(refs) != 4 {
		t.Fatalf("refs = %d, want 4", len(refs))
	}
	if refs[0].Text() != "details" || refs[0].TextDefaulted() || strings.Join(refs[0].RefPath(), "/") != "details" {
		t.Errorf("labelled ref %+v", refs[0])
	}
	if refs[1].Text() != "Details" || !refs[1].TextDefaulted() || strings.Join(refs[1].RefPath(), "/") != "details" {
		t.Errorf("defaulted ref text %q defaulted %v path %v", refs[1].Text(), refs[1].TextDefaulted(), refs[1].RefPath())
	}
	if viaPart := plan.Content()[1].Runs()[0]; refs[2].Text() != "mount" || refs[2].RefElement() != viaPart.RefElement() {
		t.Errorf("usage ref text %q element %q, a Ref part gives %q", refs[2].Text(), refs[2].RefElement(), viaPart.RefElement())
	}
	if refs[3].Text() != "Telescope" || !refs[3].TextDefaulted() || refs[3].RefElement() != "Observatory::Telescope" {
		t.Errorf("metadata ref text %q element %q", refs[3].Text(), refs[3].RefElement())
	}
	var math []Run
	for _, run := range byKind[RunSpan] {
		if run.Style() == StyleMath {
			math = append(math, run)
		}
	}
	if len(math) != 1 || math[0].Text() != `m \propto D^2` {
		t.Errorf("math runs %+v", math)
	}
}

// A body inside a paragraph part yields that paragraph: the part keeps its
// name, so it stays a reference target.
func TestCompileParagraphMarkdownBody(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part def Report :> Document {
			attribute redefines title = "Report";
			part reading : Paragraph {
				rep language "markdown" /* The model lives in [examples](https://example.com/x) and *here*. */
			}
			part see : Paragraph {
				part r : Ref { ref redefines target = reading; }
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	content := plan.Content()
	if len(content) != 2 || content[0].Name() != "reading" || content[0].Kind() != ContentParagraph {
		t.Fatalf("content %+v", content)
	}
	runs := content[0].Runs()
	if len(runs) != 5 || runs[1].Kind() != RunLink || runs[3].Style() != StyleEmphasis {
		t.Errorf("runs %+v", runs)
	}
	if ref := content[1].Runs()[0]; strings.Join(ref.RefPath(), "/") != "reading" || ref.Text() != "reading" {
		t.Errorf("ref to bodied paragraph %+v", ref)
	}
}

// A body inside an inherited paragraph is planned for the specializing
// document, and a redefinition replaces the base paragraph, body and all.
func TestCompileInheritedMarkdownBody(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part def Base :> Document {
			attribute redefines title = "Base";
			part intro : Paragraph {
				rep language "markdown" /* Inherited *prose*. */
			}
			part detail : Paragraph {
				rep language "markdown" /* Base detail. */
			}
		}
		part def Report :> Base {
			attribute redefines title = "Report";
			part redefines detail : Paragraph {
				:>> text = "Redefined detail.";
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	content := plan.Content()
	if len(content) != 2 {
		t.Fatalf("content = %d, want 2: %+v", len(content), content)
	}
	texts := []string{strings.Join(runTexts(content[1].Runs()), ""), content[0].Text()}
	if texts[0] != "Inherited prose." || texts[1] != "Redefined detail." {
		t.Errorf("texts %q", texts)
	}
	if content[0].Name() != "detail" || content[1].Name() != "intro" {
		t.Errorf("names %q %q", content[0].Name(), content[1].Name())
	}
}

func runTexts(runs []Run) []string {
	out := make([]string, len(runs))
	for i, run := range runs {
		out[i] = run.Text()
	}
	return out
}

// Each misuse of a body is a typed planning error naming the body line.
func TestCompileMarkdownBodyErrors(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		kind     ErrorKind
		message  string
		contains string
	}{
		{
			name:     "unsupported_language",
			body:     `rep language "asciidoc" /* = Title */`,
			kind:     ErrorUnsupportedMarkupLanguage,
			contains: `in "asciidoc", which no document markup language matches (supported: markdown)`,
		},
		{
			name:     "heading",
			body:     "rep language \"markdown\" /* Fine.\n *\n * # Heading\n */",
			kind:     ErrorUnsupportedMarkupConstruct,
			contains: "markdown body with an level 1 heading, which no content block renders (body line 3)",
		},
		{
			name:     "image",
			body:     "rep language \"markdown\" /* ![alt](pic.png) */",
			kind:     ErrorUnsupportedMarkupConstruct,
			contains: "an image, which no content block renders (body line 1)",
		},
		{
			name:     "nested_list",
			body:     "rep language \"markdown\" /* - a\n *   - b\n */",
			kind:     ErrorUnsupportedMarkupConstruct,
			contains: "nested list, which no content block renders (body line 2)",
		},
		{
			name: "paragraph_rep_and_text",
			body: "part p : Paragraph {\n :>> text = \"t\";\n rep language \"markdown\" /* body */\n }",
			kind: ErrorConflictingRuns,
		},
		{
			name: "paragraph_rep_and_runs",
			body: "part p : Paragraph {\n part s : Span { :>> text = \"t\"; }\n rep language \"markdown\" /* body */\n }",
			kind: ErrorConflictingRuns,
		},
		{
			name:     "paragraph_rep_yields_list",
			body:     "part p : Paragraph {\n rep language \"markdown\" /* - a\n * - b\n */\n }",
			kind:     ErrorMarkupBodyNotParagraph,
			contains: "markdown body that yields a list rather than exactly one paragraph",
		},
		{
			name:     "paragraph_rep_yields_two",
			body:     "part p : Paragraph {\n rep language \"markdown\" /* a\n *\n * b\n */\n }",
			kind:     ErrorMarkupBodyNotParagraph,
			contains: "yields 2 blocks (paragraph, paragraph)",
		},
		{
			name:     "paragraph_two_reps",
			body:     "part p : Paragraph {\n rep language \"markdown\" /* a */\n rep language \"markdown\" /* b */\n }",
			kind:     ErrorMarkupBodyNotParagraph,
			contains: "yields 2 bodies",
		},
		{
			name:     "paragraph_rep_empty",
			body:     "part p : Paragraph {\n rep language \"markdown\" /* */\n }",
			kind:     ErrorMarkupBodyNotParagraph,
			contains: "yields no content",
		},
		{
			name:     "misplaced_in_span",
			body:     "part p : Paragraph {\n part s : Span { :>> text = \"t\"; rep language \"markdown\" /* x */ }\n }",
			kind:     ErrorMisplacedMarkupBody,
			contains: "carries a markdown body, which only a document, section or paragraph may",
		},
		{
			name: "misplaced_in_formula",
			body: "part f : Formula {\n :>> source = \"x\";\n rep language \"markdown\" /* x */\n }",
			kind: ErrorMisplacedMarkupBody,
		},
		{
			name:     "unknown_ref",
			body:     "rep language \"markdown\" /* Fine.\n *\n * See [x](sysml:nowhere). */",
			kind:     ErrorUnknownRefTarget,
			contains: "names unknown target nowhere (body line 3)",
		},
		{
			name:     "invalid_ref",
			body:     "rep language \"markdown\" /* See [x](sysml:1+2). */",
			kind:     ErrorInvalidRefTarget,
			contains: `got "sysml:1+2" (body line 1)`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := loadPlanningFixture(t, `
				part def Report :> Document {
					attribute redefines title = "Report";
					`+tc.body+`
				}
			`)
			_, err := fixture.compile(t, "Report")
			planning := bodyPlanningError(t, err)
			if planning.Kind != tc.kind {
				t.Fatalf("kind %s (%v), want %s", planning.Kind, err, tc.kind)
			}
			if tc.contains != "" && !strings.Contains(err.Error(), tc.contains) {
				t.Errorf("message %q lacks %q", err.Error(), tc.contains)
			}
			if !planning.Origin.Located() {
				t.Errorf("error %v has no origin", err)
			}
		})
	}
}
