package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestCommittedKeywordsAreCurrent is the drift gate: the keyword lists the mode
// ships must be what the current lexer generates.
func TestCommittedKeywordsAreCurrent(t *testing.T) {
	want, err := Render()
	if err != nil {
		t.Fatalf("Render err = %v", err)
	}
	path := filepath.Join("..", "..", File)
	got, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != string(want) {
		t.Errorf("%s is stale; regenerate it with `make emacs-keywords`", path)
	}
}

var quoted = regexp.MustCompile(`"([^"]*)"`)

// section returns the words quoted in the list headed by head.
func section(t *testing.T, text, head string) map[string]bool {
	t.Helper()
	start := strings.Index(text, "("+head+" ")
	if start < 0 {
		t.Fatalf("no (%s ...) list in the generated file", head)
	}
	end := strings.Index(text[start:], ")")
	words := map[string]bool{}
	for _, m := range quoted.FindAllStringSubmatch(text[start:start+end], -1) {
		words[m[1]] = true
	}
	return words
}

func TestEveryKeywordIsListed(t *testing.T) {
	data, err := Render()
	if err != nil {
		t.Fatalf("Render err = %v", err)
	}
	text := string(data)
	groups := text[strings.Index(text, "(defconst sysml-mode-keyword-groups"):strings.Index(text, "(defconst sysml-mode-contextual-words")]
	listed := map[string]bool{}
	for _, m := range quoted.FindAllStringSubmatch(groups, -1) {
		listed[m[1]] = true
	}
	for _, kw := range source.Keywords() {
		if !listed[kw] {
			t.Errorf("keyword %q is in no generated group", kw)
		}
	}
}

func TestContextualWordsArePerLanguage(t *testing.T) {
	data, err := Render()
	if err != nil {
		t.Fatalf("Render err = %v", err)
	}
	for _, lang := range languages {
		got := section(t, string(data), lang.symbol)
		for _, w := range lexer.ContextualWords(lang.kind) {
			if !got[w] {
				t.Errorf("%s: contextual word %q is not listed", lang.symbol, w)
			}
		}
		if got["var"] != (lang.kind == source.KindKerML) {
			t.Errorf("%s lists var = %v, want %v", lang.symbol, got["var"], lang.kind == source.KindKerML)
		}
	}
}

func TestListsWrapAtTheLineWidth(t *testing.T) {
	data, err := Render()
	if err != nil {
		t.Fatalf("Render err = %v", err)
	}
	for i, line := range strings.Split(string(data), "\n") {
		if len(line) > lineWidth && strings.Contains(line, `"`) && !strings.HasPrefix(line, ";;;") {
			t.Errorf("line %d is %d columns wide: %s", i+1, len(line), line)
		}
	}
}
