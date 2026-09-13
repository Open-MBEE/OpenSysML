package lexer

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// The list exists so highlighting and completion know these words; letting one
// into Keywords() would reserve it and stop models naming features with it.
func TestContextualWordsAreNotReserved(t *testing.T) {
	for _, kind := range []source.Kind{source.KindSysML, source.KindKerML, source.KindUnknown} {
		for _, w := range ContextualWords(kind) {
			if IsKeyword(w) {
				t.Errorf("contextual word %q is reserved by Keywords(); it must be one or the other", w)
			}
			if !IsIdentifier(w) {
				t.Errorf("contextual word %q is not writable as a basic name", w)
			}
		}
	}
}

func TestContextualWordsDifferPerLanguage(t *testing.T) {
	sysml := ContextualWords(source.KindSysML)
	kerml := ContextualWords(source.KindKerML)
	if has(sysml, "var") {
		t.Error("`var` is offered for SysML; it is a literal of KerML.xtext only")
	}
	if !has(kerml, "var") {
		t.Error("`var` is not offered for KerML; KerML.xtext BasicFeaturePrefix takes it")
	}
	for _, w := range []string{"chain", "defer"} {
		if !has(sysml, w) || !has(kerml, w) {
			t.Errorf("%q is read as syntax in both languages but is not offered in both", w)
		}
	}
	// `on` is a literal in neither pinned grammar and syntax in no position of
	// ours, so it is a plain name and must not be offered as syntax. `initial`,
	// `region` and `point` are the same now that neither the markers they spelled,
	// the orthogonal-region member nor the entry/exit point pseudostate is accepted.
	for _, w := range []string{"on", "initial", "region", "final", "point"} {
		if has(sysml, w) || has(kerml, w) {
			t.Errorf("%q is offered as a contextual word; it is an ordinary name", w)
		}
	}
}

// KindUnknown has no file name to go by, so it gets the union.
func TestContextualWordsUnknownKindIsTheUnion(t *testing.T) {
	union := ContextualWords(source.KindUnknown)
	for _, kind := range []source.Kind{source.KindSysML, source.KindKerML} {
		for _, w := range ContextualWords(kind) {
			if !has(union, w) {
				t.Errorf("unknown-kind list omits %q, offered for %v", w, kind)
			}
		}
	}
}

func TestContextualWordsIsSortedCopy(t *testing.T) {
	got := ContextualWords(source.KindKerML)
	for i := 1; i < len(got); i++ {
		if got[i-1] >= got[i] {
			t.Fatalf("ContextualWords() is not in name order: %q before %q", got[i-1], got[i])
		}
	}
	got[0] = "MUTATED"
	if ContextualWords(source.KindKerML)[0] == "MUTATED" {
		t.Error("ContextualWords() leaked its internal slice")
	}
}

func has(words []string, w string) bool {
	for _, x := range words {
		if x == w {
			return true
		}
	}
	return false
}
