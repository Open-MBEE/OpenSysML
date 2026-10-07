package repl

import (
	"reflect"
	"testing"
)

func TestStatementsSplitACellAsThePromptReadsIt(t *testing.T) {
	cases := []struct {
		name string
		cell string
		want []Statement
	}{
		{"empty", "", nil},
		{"blank lines only", "\n  \n", nil},
		{"one meta", "%help", []Statement{{Meta: true, Text: "%help"}}},
		{"meta with surrounding space", "  %eval 1 + 2  ", []Statement{{Meta: true, Text: "%eval 1 + 2"}}},
		{"one declaration", "part def A;", []Statement{{Text: "part def A;"}}},
		{"declaration over lines with a blank line", "package P {\n\n  part def A;\n}", []Statement{{Text: "package P {\n\n  part def A;\n}"}}},
		{"meta then declaration", "%clear\npart def A;", []Statement{{Meta: true, Text: "%clear"}, {Text: "part def A;"}}},
		{"declaration then meta", "part def A;\n%print A", []Statement{{Text: "part def A;"}, {Meta: true, Text: "%print A"}}},
		{"meta inside an open brace is text", "package P {\n%notacommand\n}", []Statement{{Text: "package P {\n%notacommand\n}"}}},
		{"meta after a closed brace is a command", "package P {\n}\n%print P", []Statement{{Text: "package P {\n}"}, {Meta: true, Text: "%print P"}}},
		{"string holding a brace", "attribute s = \"{\";\n%print s", []Statement{{Text: "attribute s = \"{\";"}, {Meta: true, Text: "%print s"}}},
		{"two declarations are one submission", "part def A;\npart def B;", []Statement{{Text: "part def A;\npart def B;"}}},
		{"expression", "1 + 2", []Statement{{Text: "1 + 2"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Statements(tc.cell); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Statements(%q)\n got %#v\nwant %#v", tc.cell, got, tc.want)
			}
		})
	}
}

func TestNeedsContinuationIsTheLexersVerdict(t *testing.T) {
	for src, want := range map[string]bool{
		"package P {":           true,
		"package P {}":          false,
		"calc f(x : Real":       true,
		"attribute a = [1, 2":   true,
		"attribute a = \"{\";":  false,
		"// {\npart def A;":     false,
		"part def A; /* { */":   false,
		"package P { part x; }": false,
	} {
		if got := NeedsContinuation(src); got != want {
			t.Errorf("NeedsContinuation(%q) = %v, want %v", src, got, want)
		}
	}
}

func TestBareExpressionTellsAnExpressionFromADeclaration(t *testing.T) {
	if expr, ok := BareExpression("1 + 2"); !ok || expr != "1 + 2" {
		t.Errorf("BareExpression(1 + 2) = %q, %v", expr, ok)
	}
	if _, ok := BareExpression("part def A;"); ok {
		t.Error("a declaration read as a bare expression")
	}
	if !IsMeta("%help") || IsMeta("help") {
		t.Error("IsMeta does not read the % prefix")
	}
}

func TestKnownMetaNamesTheServedCommands(t *testing.T) {
	for _, name := range []string{"%help", "%eval", "%print", "%render", "%quit"} {
		if !KnownMeta(name) {
			t.Errorf("KnownMeta(%q) = false", name)
		}
	}
	for _, name := range []string{"%nosuch", "eval", ""} {
		if KnownMeta(name) {
			t.Errorf("KnownMeta(%q) = true", name)
		}
	}
}

func TestMetaArgsKeepsAQuotedArgumentWhole(t *testing.T) {
	got := MetaArgs(`  %render v mermaid palette "a b"  `)
	want := []string{"%render", "v", "mermaid", "palette", "a b"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MetaArgs = %q, want %q", got, want)
	}
}

func TestFailedReportsAnErrorNotAWarning(t *testing.T) {
	s := NewSession()
	if r := s.Submit("part def A;"); r.Failed(s.Verbosity()) {
		t.Errorf("a clean declaration failed: %v", RenderResult(r, s.Verbosity()))
	}
	if r := s.Submit("part def B { attribute x : Nope; }"); !r.Failed(s.Verbosity()) {
		t.Error("an unresolved reference did not fail")
	}
	if r := s.Submit("part def C;"); r.Failed(s.Verbosity()) {
		t.Errorf("the declaration after a failure failed: %v", RenderResult(r, s.Verbosity()))
	}
}
