package repl

import (
	"strings"
	"testing"
)

func TestExplainCodeExplainsAndLists(t *testing.T) {
	s := NewSession()
	if got := s.doExplainCode([]string{"unresolved"}); len(got) == 0 || !strings.HasPrefix(got[0], "unresolved") {
		t.Fatalf("%%explain-code unresolved = %q", got)
	}
	listing := strings.Join(s.doExplainCode(nil), "\n")
	for _, code := range []string{"syntax", "unresolved", "name-conflict", "document-plan-missing-title"} {
		if !strings.Contains(listing, "  "+code+" ") {
			t.Errorf("bare %%explain-code lists no %s", code)
		}
	}
	mixed := s.doExplainCode([]string{"no-such", "syntax"})
	if !strings.HasPrefix(mixed[0], errPrefix) || mixed[1] != "" || !strings.HasPrefix(mixed[2], "syntax: error") {
		t.Errorf("unknown then known = %q", mixed)
	}
}
