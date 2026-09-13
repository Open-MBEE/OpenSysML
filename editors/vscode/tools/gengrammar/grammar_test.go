package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// TestCommittedGrammarsAreCurrent is the drift gate: the grammars the extension
// ships must be what the current keyword list generates.
func TestCommittedGrammarsAreCurrent(t *testing.T) {
	for _, g := range Grammars() {
		want, err := Render(g)
		if err != nil {
			t.Fatalf("Render(%s) err = %v", g.File, err)
		}
		path := filepath.Join("..", "..", "syntaxes", g.File)
		got, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if string(got) != string(want) {
			t.Errorf("%s is stale; regenerate it with `make vscode-grammar`", path)
		}
	}
}

func TestEveryKeywordIsHighlighted(t *testing.T) {
	data, err := Render(Grammars()[0])
	if err != nil {
		t.Fatalf("Render err = %v", err)
	}
	var doc struct {
		Repository map[string]struct {
			Match string `json:"match"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var matchers []*regexp.Regexp
	for name, rule := range doc.Repository {
		if !strings.HasPrefix(name, "keywords-") && name != "constants" {
			continue
		}
		re, err := regexp.Compile(strings.ReplaceAll(rule.Match, `\b`, ``))
		if err != nil {
			t.Fatalf("rule %s has an invalid regex %q: %v", name, rule.Match, err)
		}
		matchers = append(matchers, re)
	}

	for _, kw := range lexer.Keywords() {
		matched := false
		for _, re := range matchers {
			if re.FindString(kw) == kw {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("keyword %q is not matched by any generated keyword rule", kw)
		}
	}
}

// Grouping every keyword leaves no leftovers, so generation must still work
// without the "keywords-other" rule; a misspelled rule name must still fail.
func TestRenderToleratesAnEmptyLeftoverRule(t *testing.T) {
	repo, err := repository(Grammars()[0].Kind)
	if err != nil {
		t.Fatalf("repository err = %v", err)
	}
	delete(repo, "keywords-other")
	if _, err := render(Grammars()[0], repo); err != nil {
		t.Errorf("render without keywords-other err = %v, want a grammar", err)
	}

	repo["keywords-nosuch"] = pattern{Name: "keyword.other", Match: "x"}
	if _, err := render(Grammars()[0], repo); err == nil {
		t.Error("render with a rule the order does not name succeeded, want an error")
	}
}

// The contextual words are the second source of highlighting: words the parser
// reads as syntax positionally, which the lexer does not reserve.
func TestContextualWordsAreHighlighted(t *testing.T) {
	for _, g := range Grammars() {
		rule, ok := keywordRules(t, g)["keywords-contextual"]
		if !ok {
			t.Fatalf("%s has no keywords-contextual rule", g.File)
		}
		re := wordRegexp(t, rule)
		for _, w := range lexer.ContextualWords(g.Kind) {
			if re.FindString(w) != w {
				t.Errorf("%s: contextual word %q is not matched by keywords-contextual", g.File, w)
			}
		}
	}
}

// `var` is a KerML literal and no SysML production, so the two files differ.
func TestContextualWordsAreLanguageSpecific(t *testing.T) {
	for _, g := range Grammars() {
		re := wordRegexp(t, keywordRules(t, g)["keywords-contextual"])
		if got, want := re.FindString("var") == "var", g.Kind == source.KindKerML; got != want {
			t.Errorf("%s matches `var` = %v, want %v", g.File, got, want)
		}
	}
}

// A word cannot be reserved and contextual at once: were one to be added to
// lexer.Keywords(), generation must fail rather than reserve it quietly.
func TestRenderRejectsAReservedContextualWord(t *testing.T) {
	if err := checkUnreserved(lexer.ContextualWords(source.KindUnknown), map[string]bool{"defer": true}); err == nil {
		t.Error("checkUnreserved accepted a contextual word the lexer reserves, want an error")
	}
}

// keywordRules returns the generated keyword and constant rules by name.
func keywordRules(t *testing.T, g Grammar) map[string]string {
	t.Helper()
	data, err := Render(g)
	if err != nil {
		t.Fatalf("Render(%s) err = %v", g.File, err)
	}
	var doc struct {
		Repository map[string]struct {
			Match string `json:"match"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal %s: %v", g.File, err)
	}
	out := map[string]string{}
	for name, rule := range doc.Repository {
		if strings.HasPrefix(name, "keywords-") || name == "constants" {
			out[name] = rule.Match
		}
	}
	return out
}

// wordRegexp compiles a keyword rule's match, dropping the word boundaries so a
// bare word can be tested against it.
func wordRegexp(t *testing.T, match string) *regexp.Regexp {
	t.Helper()
	re, err := regexp.Compile(strings.ReplaceAll(match, `\b`, ``))
	if err != nil {
		t.Fatalf("invalid rule regex %q: %v", match, err)
	}
	return re
}

func TestScopesAreQualifiedPerLanguage(t *testing.T) {
	for _, g := range Grammars() {
		data, err := Render(g)
		if err != nil {
			t.Fatalf("Render(%s) err = %v", g.File, err)
		}
		if !strings.Contains(string(data), "keyword.declaration."+g.ScopeName) {
			t.Errorf("%s does not scope its keywords to %s", g.File, g.ScopeName)
		}
	}
}
