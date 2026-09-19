package format

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
)

var update = flag.Bool("update", false, "rewrite the .golden files in testdata")

func format(t *testing.T, src string) string {
	t.Helper()
	out, err := Source("t.sysml", []byte(src), DefaultOptions)
	if err != nil {
		t.Fatalf("Source() error = %v", err)
	}
	return string(out)
}

func TestIndentsNestedBlocks(t *testing.T) {
	got := format(t, "package P {\npart def Car {\nattribute mass;\n}\n}\n")
	want := "package P {\n    part def Car {\n        attribute mass;\n    }\n}\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestReindentsOverIndentedBlocks(t *testing.T) {
	got := format(t, "package P {\n            part def Car;\n}\n")
	if want := "package P {\n    part def Car;\n}\n"; got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestStripsTrailingWhitespaceAndCollapsesBlankLines(t *testing.T) {
	got := format(t, "package P {   \n\n\n\n    part def Car;   \n}\n")
	if want := "package P {\n\n    part def Car;\n}\n"; got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestTightensBracketPaddingAndSeparators(t *testing.T) {
	got := format(t, "package P { calc def c { return add( 1 , 2 ) ; } }\n")
	if want := "package P { calc def c { return add(1, 2); } }\n"; got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestPreservesAuthorLineBreaks(t *testing.T) {
	// A one-line block stays on one line; a split one stays split.
	if got := format(t, "package P { part def Car; }\n"); got != "package P { part def Car; }\n" {
		t.Fatalf("one-liner rewrapped: %q", got)
	}
}

func TestPreservesCommentsAndNotes(t *testing.T) {
	src := "package P {\n// a note\n/* a comment */\n//* a\n   multi-line note */\npart def Car;\n}\n"
	got := format(t, src)
	for _, want := range []string{"// a note", "/* a comment */", "multi-line note */"} {
		if !strings.Contains(got, want) {
			t.Fatalf("comment %q lost:\n%s", want, got)
		}
	}
}

// A line-comment token carries its own terminating newline, so neither the
// blank-line rule nor the final newline may add a second one.
func TestLineCommentNewlineNotDoubled(t *testing.T) {
	cases := []struct{ src, want string }{
		{"package P;\n// trailing note\n", "package P;\n// trailing note\n"},
		{"package P;\n// note\n\n\n\npart def Car;\n", "package P;\n// note\n\npart def Car;\n"},
		{"package P;\n// note\npart def Car;\n", "package P;\n// note\npart def Car;\n"},
		{"// leading note\npackage P;\n", "// leading note\npackage P;\n"},
		// The comment token's text gains the newline the formatter supplies,
		// which the token-stream check must not read as a rewrite.
		{"package P;\n// unterminated note", "package P;\n// unterminated note\n"},
	}
	for _, tc := range cases {
		if got := format(t, tc.src); got != tc.want {
			t.Errorf("format(%q):\ngot:  %q\nwant: %q", tc.src, got, tc.want)
		}
	}
}

func TestPreservesStringContents(t *testing.T) {
	src := "package P {\nattribute a = \"  spaced  ;  \";\n}\n"
	if got := format(t, src); !strings.Contains(got, `"  spaced  ;  "`) {
		t.Fatalf("string literal altered:\n%s", got)
	}
}

func TestDoesNotInsertSpacesTheAuthorOmitted(t *testing.T) {
	got := format(t, "package P { calc def c { return 1+2; } }\n")
	if !strings.Contains(got, "1+2") {
		t.Fatalf("spacing invented:\n%s", got)
	}
}

func TestUseTabs(t *testing.T) {
	out, err := Source("t.sysml", []byte("package P {\npart def Car;\n}\n"), Options{UseTabs: true})
	if err != nil {
		t.Fatal(err)
	}
	if want := "package P {\n\tpart def Car;\n}\n"; string(out) != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestUnbalancedBracesDoNotPanic(t *testing.T) {
	if format(t, "package P { part def Car; } }\n") == "" {
		t.Fatal("empty output")
	}
}

// TestCorpusIsPreservedAndStable formats every shipped library file and example
// model, asserting the token stream survives and that formatting is idempotent.
func TestCorpusIsPreservedAndStable(t *testing.T) {
	src := libs.DefaultSource()
	for _, name := range src.List() {
		content, err := src.Read(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		checkStable(t, name, content)
	}

	for _, dir := range []string{"../../../examples", "../../core/runtime/testdata"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read dir %s: %v", dir, err)
		}
		for _, e := range entries {
			ext := filepath.Ext(e.Name())
			if e.IsDir() || (ext != ".sysml" && ext != ".kerml") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			checkStable(t, path, content)
		}
	}
}

func checkStable(t *testing.T, name string, content []byte) {
	t.Helper()
	once, err := Source(name, content, DefaultOptions)
	if err != nil {
		t.Errorf("%s: %v", name, err)
		return
	}
	twice, err := Source(name, once, DefaultOptions)
	if err != nil {
		t.Errorf("%s (second pass): %v", name, err)
		return
	}
	if string(once) != string(twice) {
		t.Errorf("%s: formatting is not idempotent", name)
	}
}

// TestContinuationGolden locks the indentation of lines that continue the line
// above them, and checks the result is stable under a second pass.
func TestContinuationGolden(t *testing.T) {
	const path = "testdata/continuation.sysml"
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Source(path, src, DefaultOptions)
	if err != nil {
		t.Fatalf("Source() error = %v", err)
	}
	golden := strings.TrimSuffix(path, ".sysml") + ".golden.sysml"
	if *update {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("%s differs\n--- want ---\n%s\n--- got ---\n%s", golden, want, got)
	}
	checkStable(t, path, src)
}

// A one-level-deeper continuation is what a reader expects; the statement's own
// level would read as a new statement.
func TestIndentsContinuationLines(t *testing.T) {
	got := format(t, "package Q {\nattribute t = \"a\" +\n\"b\";\n}\n")
	want := "package Q {\n    attribute t = \"a\" +\n        \"b\";\n}\n"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

// A declaration whose body is a note ends with that note, so what follows is a
// new statement rather than a continuation.
func TestNoteBodyEndsTheStatement(t *testing.T) {
	got := format(t, "package P {\ndoc /* about P */\npart def Wheel;\n}\n")
	want := "package P {\n    doc /* about P */\n    part def Wheel;\n}\n"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

// A CRLF document keeps CRLF everywhere, including after a line comment (whose
// token text carries its own terminator) and at the end of the file.
func TestSourceKeepsWindowsLineEndings(t *testing.T) {
	src := "package P {\r\n// note\r\nattribute x = 1;\r\n}\r\n"
	out, err := Source("crlf.sysml", []byte(src), DefaultOptions)
	if err != nil {
		t.Fatalf("Source err = %v", err)
	}
	got := string(out)
	if strings.Contains(strings.ReplaceAll(got, "\r\n", ""), "\n") {
		t.Errorf("mixed line endings in output: %q", got)
	}
	if !strings.HasSuffix(got, "\r\n") || strings.HasSuffix(got, "\r\r\n") {
		t.Errorf("bad trailing line ending: %q", got)
	}
	if !strings.Contains(got, "    attribute x = 1;\r\n") {
		t.Errorf("not re-indented: %q", got)
	}
}

// A file that is mostly LF is normalized to LF rather than left mixed.
func TestSourceNormalizesMixedLineEndingsToLF(t *testing.T) {
	src := "package P {\n// note\r\nattribute x = 1;\n}\n"
	out, err := Source("mixed.sysml", []byte(src), DefaultOptions)
	if err != nil {
		t.Fatalf("Source err = %v", err)
	}
	if got := string(out); strings.Contains(got, "\r") {
		t.Errorf("carriage return survived: %q", got)
	}
}

// An unterminated block comment runs to the end of the file, so the trailing
// newline the formatter normally guarantees would land inside that comment and
// change its own text — which used to trip the idempotence check and refuse the
// save the session most needs to make.
func TestSourceKeepsAnUnterminatedComment(t *testing.T) {
	for _, src := range []string{
		"part def A;\n/* oops",
		"part def A;\n/* oops\n",
		"doc /* unclosed",
		"part def A;\n//* oops", // a documentation note, opener "//*"
		"part def A;\n/*/",      // ends with "*/" but is the opener itself
		"part def A;\n//*/",
		"package P {\n/* oops", // still open inside a block
	} {
		out, err := Source("s.sysml", []byte(src), DefaultOptions)
		if err != nil {
			t.Errorf("Source(%q) = %v, want it formatted", src, err)
			continue
		}
		comment := src[strings.Index(src, "/"):]
		if !strings.HasSuffix(string(out), comment) {
			t.Errorf("Source(%q) = %q, want it to end with the comment as typed", src, out)
		}
	}
}

// A closing bracket alone on a line belongs to the line that opened it, not one
// level in from it. Only reachable through a tolerant save, since a trailing
// comma is a syntax error.
func TestSourceIndentsAClosingBracketAtStatementLevel(t *testing.T) {
	src := "package P {\nattribute a = max(1,\n2,\n);\n}\n"
	want := "package P {\n    attribute a = max(1,\n        2,\n    );\n}\n"
	out, err := Source("s.sysml", []byte(src), DefaultOptions)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != want {
		t.Errorf("Source() =\n%s\nwant\n%s", out, want)
	}
}
