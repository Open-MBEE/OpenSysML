package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// openFilterFixtures opens the named fixtures under tests/testdata/filters as
// one workspace and returns each document's diagnostic messages by name.
func openFilterFixtures(t *testing.T, names ...string) map[string][]string {
	t.Helper()
	ws := NewWorkspace()
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join("..", "..", "..", "tests", "testdata", "filters", name))
		if err != nil {
			t.Fatalf("read fixture: %v", err)
		}
		ws.Open("file:///"+name, []byte(data), 1)
	}
	out := map[string][]string{}
	for _, name := range names {
		for _, d := range ws.Diagnostics("file:///" + name) {
			out[name] = append(out[name], d.Message)
		}
	}
	return out
}

// unresolvedNames is the names the messages report as unresolved references.
func unresolvedNames(msgs []string) []string {
	var out []string
	for _, msg := range msgs {
		if _, after, ok := strings.Cut(msg, "unresolved reference: "); ok {
			out = append(out, strings.Fields(after)[0])
		}
	}
	return out
}

// A filter names a Type element, not a spelling: an `alias` member of it and an
// import that surfaces it under another namespace select what the Type
// annotates, through an import filter, a `filter` member and a filtered expose
// alike, and reject the rest (KerML 8.3.2.3, SysML v2 7.4.4).
func TestFilterThroughAnAliasSelectsByTheAliasedType(t *testing.T) {
	for name, surfaces := range map[string]int{"alias_member.sysml": 3, "alias_import.sysml": 2} {
		got := openFilterFixtures(t, name)[name]
		unresolved := unresolvedNames(got)
		if len(got) != surfaces || len(unresolved) != surfaces {
			t.Fatalf("%s: want %d diagnostics, each an unresolved reference, got %v", name, surfaces, got)
		}
		for _, n := range unresolved {
			if n != "radio" {
				t.Errorf("%s: the alias must admit belt and reject radio only, got unresolved %s", name, n)
			}
		}
	}
}

// Two documents each opening `Meta::Safety` declare two Types that share a
// qualified name; a filter naming one selects nothing the other annotates, while
// the filter written beside the annotating declaration still selects it.
func TestFilterNamingANamesakeTypeDoesNotConform(t *testing.T) {
	got := openFilterFixtures(t, "namesake_a.sysml", "namesake_b.sysml")
	if unresolved := unresolvedNames(got["namesake_a.sysml"]); len(unresolved) != 0 {
		t.Fatalf("the filter naming the annotating Safety must admit belt, got %v", got["namesake_a.sysml"])
	}
	if unresolved := unresolvedNames(got["namesake_b.sysml"]); len(unresolved) != 1 || unresolved[0] != "belt" {
		t.Fatalf("the filter naming the other document's Safety must reject belt, got %v", got["namesake_b.sysml"])
	}
}
