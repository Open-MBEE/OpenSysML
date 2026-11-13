package keywords

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestGroupsCoverEveryKeywordOnce(t *testing.T) {
	got, err := Groups()
	if err != nil {
		t.Fatalf("Groups() err = %v", err)
	}
	seen := map[string]Category{}
	for _, g := range got {
		for _, w := range g.Words {
			if prev, ok := seen[w]; ok {
				t.Errorf("%q is in both %s and %s", w, prev, g.Category)
			}
			seen[w] = g.Category
		}
	}
	for _, kw := range source.Keywords() {
		if _, ok := seen[kw]; !ok {
			t.Errorf("keyword %q is in no group", kw)
		}
	}
}

func TestValidateRejectsAWordTheLexerDropped(t *testing.T) {
	_, err := validate([]Group{{Category: Control, Words: []string{"nosuch"}}}, []string{"if"})
	if err == nil {
		t.Error("validate accepted a word the lexer does not reserve, want an error")
	}
}

func TestValidateRejectsAWordInTwoGroups(t *testing.T) {
	in := []Group{
		{Category: Control, Words: []string{"if"}},
		{Category: Modifier, Words: []string{"if"}},
	}
	if _, err := validate(in, []string{"if"}); err == nil {
		t.Error("validate accepted a word in two groups, want an error")
	}
}

func TestValidateCollectsUngroupedWordsAsOther(t *testing.T) {
	got, err := validate([]Group{{Category: Control, Words: []string{"if"}}}, []string{"if", "then", "else"})
	if err != nil {
		t.Fatalf("validate err = %v", err)
	}
	last := got[len(got)-1]
	if last.Category != Other || len(last.Words) != 2 || last.Words[0] != "then" || last.Words[1] != "else" {
		t.Errorf("last group = %+v, want other [then else]", last)
	}
}

func TestValidateOmitsAnEmptyOther(t *testing.T) {
	got, err := validate([]Group{{Category: Control, Words: []string{"if"}}}, []string{"if"})
	if err != nil {
		t.Fatalf("validate err = %v", err)
	}
	if len(got) != 1 {
		t.Errorf("groups = %+v, want only the control group", got)
	}
}

// `var` is a KerML literal and no SysML production, so the languages differ.
func TestContextualWordsAreLanguageSpecific(t *testing.T) {
	for kind, want := range map[source.Kind]bool{source.KindSysML: false, source.KindKerML: true} {
		words, err := Contextual(kind)
		if err != nil {
			t.Fatalf("Contextual(%v) err = %v", kind, err)
		}
		has := false
		for _, w := range words {
			has = has || w == "var"
		}
		if has != want {
			t.Errorf("Contextual(%v) has var = %v, want %v", kind, has, want)
		}
	}
}

// A word cannot be reserved and contextual at once: were one to be added to
// source.Keywords(), generation must fail rather than reserve it quietly.
func TestCheckUnreservedRejectsAReservedContextualWord(t *testing.T) {
	if err := checkUnreserved(lexer.ContextualWords(source.KindUnknown), []string{"chain"}); err == nil {
		t.Error("checkUnreserved accepted a contextual word the lexer reserves, want an error")
	}
}

func TestIntroducersAreReserved(t *testing.T) {
	if _, err := Introducers(); err != nil {
		t.Fatalf("Introducers() err = %v", err)
	}
	if _, err := checkReserved([]string{"nosuch"}, []string{"def"}); err == nil {
		t.Error("checkReserved accepted an unreserved introducer, want an error")
	}
}

// `library package Foo` names Foo, not `package`: only words that are directly
// followed by a name may introduce one.
func TestLibraryIsNotAnIntroducer(t *testing.T) {
	words, err := Introducers()
	if err != nil {
		t.Fatalf("Introducers() err = %v", err)
	}
	for _, w := range words {
		if w == "library" {
			t.Error("library is an introducer, but it is followed by `package`, not a name")
		}
	}
}
