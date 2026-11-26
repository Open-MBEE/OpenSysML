package jupyter

import (
	"slices"
	"strings"
	"testing"
)

func TestACommandAnsweredWithItsUsageFails(t *testing.T) {
	for _, code := range []string{
		"%save",
		"%validate a b",
		"%instantiate",
		"%render-run",
		"%render-run nope",
		"%clock-step 1 2",
	} {
		got := mustFail(t, newEngine(t), code)
		if got.Name != "UsageError" || got.Value == "" {
			t.Errorf("Execute(%q) failed as %s (%q), want a UsageError", code, got.Name, got.Value)
		}
		if !slices.ContainsFunc(got.Traceback, func(line string) bool { return strings.Contains(line, "usage: %") }) {
			t.Errorf("Execute(%q) failed without its usage line: %q", code, got.Traceback)
		}
	}
	if got := mustFail(t, newEngine(t), "%features x depth nope"); got.Name != "CommandError" {
		t.Errorf("an error printed beside its usage failed as %s, want a CommandError", got.Name)
	}
	if out := mustRun(t, newEngine(t), "%help").text(); !strings.Contains(out, "%save") {
		t.Errorf("%%help printed %q", out)
	}
	if out := mustRun(t, newEngine(t), `%eval "a; usage: %b"`).text(); !strings.Contains(out, "usage: %b") {
		t.Errorf("an expression quoting a usage line printed %q", out)
	}
}
