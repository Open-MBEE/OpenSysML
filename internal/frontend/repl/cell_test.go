package repl

import (
	"reflect"
	"testing"
)

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
