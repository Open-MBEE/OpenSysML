package repl

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
)

const replTypo = "package P { state def M { entry; then a; state a; state b; transition first a when Pnig then b; } }"

func lintFindings(s *Session) int {
	n := 0
	for _, d := range s.Diagnostics() {
		if d.Code == passes.CodeUndeclaredSignal {
			n++
		}
	}
	return n
}

func TestLintMetaCommandSwitchesALint(t *testing.T) {
	s := NewSession()
	s.Submit(replTypo)
	if lintFindings(s) != 1 {
		t.Fatalf("want the lint on by default: %v", s.Diagnostics())
	}
	if s.HasErrors() {
		t.Fatal("a lint warning counted as an error")
	}
	out := strings.Join(meta(t, s, "%lint"), "\n")
	if !strings.Contains(out, "lint undeclared-signal: on") || !strings.Contains(out, "lint port-type-mismatch: on") {
		t.Fatalf("%%lint = %q", out)
	}
	meta(t, s, "%lint undeclared-signal off")
	if lintFindings(s) != 0 {
		t.Fatalf("%%lint off kept the finding: %v", s.Diagnostics())
	}
	if got := s.DisabledLints(); len(got) != 1 || got[0] != passes.CodeUndeclaredSignal {
		t.Fatalf("DisabledLints = %v", got)
	}
	meta(t, s, "%lint undeclared-signal on")
	if lintFindings(s) != 1 {
		t.Fatal("%lint on did not bring the finding back")
	}
	for _, bad := range []string{"%lint bogus off", "%lint undeclared-signal maybe", "%lint undeclared-signal"} {
		if got := meta(t, s, bad); len(got) == 0 || !strings.HasPrefix(got[0], "error:") {
			t.Fatalf("%s = %v, want an error", bad, got)
		}
	}
}

// Strict conformance escalates notation, never a lint.
func TestStrictModeLeavesLintsWarnings(t *testing.T) {
	s := NewSession()
	meta(t, s, "%strict on")
	s.Submit(replTypo)
	if lintFindings(s) != 1 || s.HasErrors() {
		t.Fatalf("strict mode changed the lint: %v", s.Diagnostics())
	}
}

// An opt-in lint lists as off and reports nothing until %lint switches it on.
func TestLintMetaCommandSwitchesAnOptInLint(t *testing.T) {
	s := NewSession()
	s.Submit("private import ScalarValues::*; attribute x : Real = 0.1;")
	rounded := func() int {
		n := 0
		for _, d := range s.Diagnostics() {
			if d.Code == passes.CodeRoundedRealLiteral {
				n++
			}
		}
		return n
	}
	if rounded() != 0 {
		t.Fatalf("want the opt-in lint off by default: %v", s.Diagnostics())
	}
	if out := strings.Join(meta(t, s, "%lint"), "\n"); !strings.Contains(out, "lint rounded-real-literal: off") ||
		!strings.Contains(out, "lint undeclared-signal: on") {
		t.Fatalf("%%lint = %q", out)
	}
	out := strings.Join(meta(t, s, "%lint rounded-real-literal on"), "\n")
	if rounded() != 1 || !strings.Contains(out, "lint rounded-real-literal: on") {
		t.Fatalf("%%lint on = %q, diagnostics %v", out, s.Diagnostics())
	}
	if got := s.EnabledLints(); len(got) != 1 || got[0] != passes.CodeRoundedRealLiteral {
		t.Fatalf("EnabledLints = %v", got)
	}
	meta(t, s, "%lint rounded-real-literal off")
	if rounded() != 0 || len(s.EnabledLints()) != 0 || len(s.DisabledLints()) != 0 {
		t.Fatalf("%%lint off left enabled %v, disabled %v, diagnostics %v", s.EnabledLints(), s.DisabledLints(), s.Diagnostics())
	}
}
