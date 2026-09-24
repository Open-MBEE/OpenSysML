package repl

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/suggest"
)

// A name the session itself declares under a package is nearer than a library
// name spelled the same, and is offered first.
func TestQualifiedSuggestionsPreferTheSession(t *testing.T) {
	s := NewSession()
	if res := s.Submit("package Mine {\n    part def Integer;\n}"); len(res.Diagnostics) > 0 {
		t.Fatalf("declaration has diagnostics: %v", res.Diagnostics)
	}
	got := s.qualifiedSuggestions(s.browseIndex(), "Integer")
	if len(got) == 0 || got[0] != "Mine::Integer" {
		t.Errorf("got = %v, want the session's own Mine::Integer first", got)
	}
}

// A member of a library function — a parameter such as RealFunctions::im::x — is
// no use as a suggestion, so a name held by a package outranks it.
func TestQualifiedSuggestionsPreferPackageMembers(t *testing.T) {
	s := NewSession()
	s.Submit("part def A;")
	got := s.qualifiedSuggestions(s.browseIndex(), "length")
	if len(got) == 0 {
		t.Fatal("no suggestion for length")
	}
	if got[0] != "ISQBase::length" {
		t.Errorf("got = %v, want the package member ISQBase::length first", got)
	}
}

// However many same-named candidates the library holds, the message offers a
// bounded few of them.
func TestQualifiedSuggestionsAreCapped(t *testing.T) {
	s := NewSession()
	s.Submit("part def A;")
	for _, name := range []string{"x", "value", "length", "Integer", "Real"} {
		if got := s.qualifiedSuggestions(s.browseIndex(), name); len(got) > suggest.Limit {
			t.Errorf("%s offered %d candidates, want at most %d: %v", name, len(got), suggest.Limit, got)
		}
	}
}

// The ranking reaches the message the prompt prints for a name it cannot find.
func TestUnresolvedMessageOffersRankedNames(t *testing.T) {
	s := NewSession()
	s.Submit("part def A;")
	_, _, err := s.lookupSymbol("length")
	if err == nil {
		t.Fatal("a bare length resolved")
	}
	if !strings.Contains(err.Error(), "ISQBase::length") {
		t.Errorf("err = %v, want it to offer ISQBase::length", err)
	}
	offered, _, _ := strings.Cut(err.Error(), "?")
	_, list, _ := strings.Cut(offered, "did you mean ")
	if n := strings.Count(list, ",") + 1; n > suggest.Limit {
		t.Errorf("err = %v, offered %d candidates, want at most %d", err, n, suggest.Limit)
	}
}

// An operator member cannot be written as a name, so it is never offered.
func TestQualifiedSuggestionsSkipUnwritableNames(t *testing.T) {
	if writableName("BaseFunctions::#::index") {
		t.Error("an operator segment was taken for a writable name")
	}
	if !writableName("ScalarValues::Integer") {
		t.Error("an ordinary qualified name was rejected")
	}
}
