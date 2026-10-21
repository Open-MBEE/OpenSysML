package specexamples

import (
	"strings"
	"testing"
)

// layout is a miniature `pdftotext -layout` rendering: a contents page on a
// roman-numbered page, then body pages whose running header carries the page
// number on either side, with examples, prose, bullets and a notation table.
const layout = "" +
	"                    Systems Modeling Language v2.1, Beta 2, Part 1                    iii\n" +
	"7 Language Description ............ 15\n" +
	"7.4.2 Comments ..................... 20\n" +
	"\f20                    Systems Modeling Language v2.1, Beta 2, Part 1\n" +
	"7 Language Description\n" +
	"7.4.2 Comments and Documentation\n" +
	"Prose at the margin describes the example.\n" +
	"\n" +
	"       package P {\n" +
	"           comment C /* about P */\n" +
	"\n" +
	"           /* also about P */\n" +
	"       }\n" +
	"\n" +
	"       • A bullet; with a semicolon\n" +
	"       • Another bullet {\n" +
	"\n" +
	"       Element                 Graphical Notation            Textual Notation\n" +
	"\n" +
	"       attribute a : Real;\n" +
	"       part def V {\n" +
	"\f                    Systems Modeling Language v2.1, Beta 2, Part 1                    21\n" +
	"           attribute b : Real;\n" +
	"       }\n" +
	"\n" +
	"                                   Package1                      package Package1 {\n" +
	"                                                                 }\n" +
	"\n" +
	"       part def Abridged {\n" +
	"           ...\n" +
	"       }\n" +
	"\n" +
	"       part def Open {\n" +
	"\n" +
	"7.4.3 Next\n" +
	"       // KerML\n" +
	"       class K; // a { in a comment\n" +
	"7.4.4 Prose\n" +
	"       If the name given for an invocation resolves to an expression instead of a function,\n" +
	"       then the invocation is taken to be for the function of that expression (see 7.3 ).\n" +
	"       function F { in x : Anything; }\n" +
	"       Note that the referenced feature may be an expression rather than a function here.\n" +
	"\n" +
	"       Name Compartment                 «abstract part def»                 abstract part def\n" +
	"       - Definition                        PartDef1                         PartDef1;\n" +
	"\n" +
	"       Package (name in                                            package Package1;\n" +
	"       body)                              Package1\n" +
	"\n" +
	"       action loop2                  for v : T in vals {\n" +
	"       }\n" +
	"8.2.2 Grammar\n" +
	"       Rule : 'kw' { x = 1 };\n" +
	"8.4.1 Semantics\n" +
	"       part def S;\n"

func TestExtractRules(t *testing.T) {
	res := Extract(Documents[0], layout)
	if res.Title != "Systems Modeling Language v2.1, Beta 2, Part 1" {
		t.Errorf("title %q", res.Title)
	}
	var got []string
	for _, b := range res.Blocks {
		got = append(got, b.Path()+"@"+b.Page+":"+strings.Join(b.Lines, "|"))
	}
	want := []string{
		"sysml/7.4.2/1.sysml@20:package P {|    comment C /* about P */||    /* also about P */|}",
		"sysml/7.4.2/2.sysml@20:attribute a : Real;|part def V {|    attribute b : Real;|}",
		"sysml/7.4.3/1.kerml@21:// KerML|class K; // a { in a comment",
		"sysml/7.4.4/1.sysml@21:function F { in x : Anything; }",
		"sysml/7.4.4/2.sysml@21:action loop2                  for v : T in vals {|}",
		"sysml/8.4.1/1.sysml@21:part def S;",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("blocks:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	var excluded []string
	for _, e := range res.Excluded {
		excluded = append(excluded, e.Clause+" "+e.Reason+" ("+e.First+")")
	}
	wantExcluded := []string{
		"7.4.2 abridged with ... (part def Abridged {)",
		"7.4.2 unbalanced braces (part def Open {)",
		"7.4.4 notation table (Name Compartment                 «abstract part def»                 abstract part def)",
		"7.4.4 notation table (Package (name in                                            package Package1;)",
	}
	if strings.Join(excluded, "\n") != strings.Join(wantExcluded, "\n") {
		t.Errorf("excluded:\n%s\nwant:\n%s", strings.Join(excluded, "\n"), strings.Join(wantExcluded, "\n"))
	}
}

func TestExtractIsDeterministic(t *testing.T) {
	a := Extract(Documents[0], layout)
	b := Extract(Documents[0], layout)
	if len(a.Blocks) != len(b.Blocks) {
		t.Fatalf("%d vs %d blocks", len(a.Blocks), len(b.Blocks))
	}
	for i := range a.Blocks {
		if a.Blocks[i].Path() != b.Blocks[i].Path() || strings.Join(a.Blocks[i].Lines, "\n") != strings.Join(b.Blocks[i].Lines, "\n") {
			t.Errorf("block %d differs between runs", i)
		}
	}
}

func TestExtractIgnoresFrontMatter(t *testing.T) {
	front := "                    Kernel Modeling Language v1.1, Beta 2                    iii\n" +
		"7 Language Description ............ 15\n" +
		"       classifier InContents;\n"
	if res := Extract(Documents[1], front); len(res.Blocks) != 0 || len(res.Excluded) != 0 {
		t.Errorf("front matter yielded %d block(s) and %d exclusion(s)", len(res.Blocks), len(res.Excluded))
	}
}

func TestRenderCompletion(t *testing.T) {
	b := Block{Document: "sysml", Clause: "7.4.2", Page: "20", Index: 2, Ext: "sysml", Lines: []string{"attribute a : Real;", "", "part def V;"}}
	got := Render("Systems Modeling Language v2.1, Beta 2, Part 1", "2026-08", b, []string{"ScalarValues::Real"})
	want := "// Systems Modeling Language v2.1, Beta 2, Part 1 (release 2026-08), clause 7.4.2, page 20, example 2.\n" +
		"// Completion: wrapped in a package; imports ScalarValues::Real for names the example leaves undeclared.\n" +
		"package 'Sysml 7.4.2 example 2' {\n" +
		"    private import ScalarValues::Real;\n" +
		"\n" +
		"    attribute a : Real;\n" +
		"\n" +
		"    part def V;\n" +
		"}\n"
	if got != want {
		t.Errorf("rendered:\n%s\nwant:\n%s", got, want)
	}
}

func TestActionBodyWrapper(t *testing.T) {
	cases := map[string]string{
		"first start then a1;":                  "package",
		"succession first a1 then a2;":          "package",
		"first start;\nthen a1;":                "action def",
		"merge m;\nfirst m then a1;":            "action def",
		"action a1 { first start; }":            "package",
		"if x > 1 {\n    action a1;\n}":         "action def",
		"part def P {\n    attribute if;\n}":    "package",
		"// for a comment\naction a1; // if {":  "package",
		"for i in 1..3 {\n    action a1;\n}":    "action def",
		"action def A { in x; }\nsend x via p;": "action def",
	}
	for text, want := range cases {
		b := Block{Lines: strings.Split(text, "\n")}
		if got := b.Wrapper(); got != want {
			t.Errorf("%q: wrapper %q, want %q", text, got, want)
		}
	}
}

func TestCompleteImportsUniqueLibraryMembers(t *testing.T) {
	blocks := []Block{
		{Document: "sysml", Clause: "7.4.2", Page: "20", Index: 1, Ext: "sysml", Lines: []string{"attribute a : Real;", "part p : Part;", "part q : Undeclared;", "attribute m = 1 [kg];", "attribute s = size(p);", "part def A :> B;", "state s2 : StateActivity;"}},
		{Document: "kerml", Clause: "7.3.4", Page: "30", Index: 1, Ext: "kerml", Lines: []string{"feature f : Boolean;"}},
	}
	fixtures := Complete("Title", "2026-08", blocks)
	if got := strings.Join(fixtures[0].Imports, ","); got != "Parts::Part,SI::kg,ScalarValues::Real" {
		t.Errorf("sysml imports %q (size is declared by more than one library package, kg is a short name in a unit position, B is a short name outside one, StateActivity is an extension library)", got)
	}
	if got := strings.Join(fixtures[1].Imports, ","); got != "ScalarValues::Boolean" {
		t.Errorf("kerml imports %q", got)
	}
	if !strings.Contains(fixtures[0].Content, "    private import Parts::Part;\n    private import SI::kg;\n    private import ScalarValues::Real;\n") {
		t.Errorf("imports missing from content:\n%s", fixtures[0].Content)
	}
	if strings.Contains(fixtures[0].Content, "Undeclared;\n    private") {
		t.Errorf("import placed after the example:\n%s", fixtures[0].Content)
	}
}

// A block comment that spans lines hides its braces and ellipses from the
// depth count and the completeness checks, like a one-line comment does.
func TestExtractMultilineComment(t *testing.T) {
	text := "" +
		"\f20                    Systems Modeling Language v2.1, Beta 2, Part 1\n" +
		"7 Language Description\n" +
		"7.4.2 Comments and Documentation\n" +
		"\n" +
		"       part def P {\n" +
		"           /* a note that mentions ... and {\n" +
		"              an unbalanced } brace */\n" +
		"           attribute x : Real;\n" +
		"       }\n" +
		"\n" +
		"Prose follows.\n"
	res := Extract(Documents[0], text)
	if len(res.Excluded) != 0 {
		t.Errorf("excluded %+v", res.Excluded)
	}
	if len(res.Blocks) != 1 || len(res.Blocks[0].Lines) != 5 {
		t.Fatalf("blocks %+v", res.Blocks)
	}
}
