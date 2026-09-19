package repl

import (
	"strings"
	"testing"
)

// A submission that no longer states a wildcard import must not go on resolving
// the names that import surfaced. The session keeps one index across
// submissions, so this is what makes reuse safe rather than stale.
func TestSubmitDropsAReplacedWildcardImport(t *testing.T) {
	s := NewSession()
	s.Submit("package Lib { part def Widget; }")
	s.Submit("package P { public import Lib::*; }")

	if _, _, err := s.lookupSymbol("P::Widget"); err != nil {
		t.Fatalf("P::Widget after importing Lib::*: %v", err)
	}
	indexed := s.symbolIndex()

	s.Submit("package P { }")
	if _, _, err := s.lookupSymbol("P::Widget"); err == nil {
		t.Error("P::Widget still resolves after the submission dropping `import Lib::*`")
	}
	if _, _, err := s.lookupSymbol("Lib::Widget"); err != nil {
		t.Errorf("Lib::Widget after the import was dropped: %v", err)
	}
	if s.symbolIndex() != indexed {
		t.Error("the session rebuilt its index instead of updating the one it had")
	}
}

// Re-stating the import surfaces the names again.
func TestSubmitRestoresAReinstatedWildcardImport(t *testing.T) {
	s := NewSession()
	s.Submit("package Lib { part def Widget; }")
	s.Submit("package P { public import Lib::*; }")
	s.Submit("package P { }")
	s.Submit("package P { public import Lib::*; }")

	if _, _, err := s.lookupSymbol("P::Widget"); err != nil {
		t.Errorf("P::Widget after re-stating the import: %v", err)
	}
}

// The session index carries the standard library, so a measurement unit named
// by a quantity expression resolves at the prompt.
func TestSessionIndexResolvesLibraryUnit(t *testing.T) {
	s := NewSession()
	s.Submit("package P { import SI::*; attribute speed = 1.5 [m/s]; }")
	out, _, err := s.runMeta("%eval P::speed")
	if err != nil {
		t.Fatalf("%%eval P::speed: %v", err)
	}
	if got := strings.Join(out, "\n"); !strings.Contains(got, "m/s") {
		t.Fatalf("want the unit in the value, got %q", got)
	}
}

// Redeclaring without an import drops the names it brought in: re-indexing the
// document takes back what an earlier import re-exported.
func TestSessionIndexDropsRemovedImport(t *testing.T) {
	s := NewSession()
	s.Submit("package P { import SI::*; }")
	if _, _, err := s.lookupSymbol("P::metre"); err != nil {
		t.Fatalf("imported name should resolve: %v", err)
	}
	s.Submit("package P { }")
	if _, fqn, err := s.lookupSymbol("P::metre"); err == nil {
		t.Fatalf("removed import still resolves as %q", fqn)
	}
}
