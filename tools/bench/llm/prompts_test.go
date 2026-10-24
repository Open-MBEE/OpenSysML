package llm

import (
	"strings"
	"testing"
)

func TestPromptsAndExtraction(t *testing.T) {
	task := Task{ID: "X", Task: "Model a package `Demo`."}
	if p := Primer(task); !strings.Contains(p, task.Task) || !strings.Contains(p, "private import ScalarValues::*;") {
		t.Errorf("primer:\n%s", p)
	}
	fb := Feedback([]Diagnostic{{File: "model.sysml", Line: 3, Column: 9, Severity: "error", Code: UnresolvedCode, Message: "unresolved reference: Seat"}})
	if !strings.Contains(fb, "model.sysml:3:9: error[unresolved]: unresolved reference: Seat") || !strings.Contains(fb, "1 diagnostic(s)") {
		t.Errorf("feedback:\n%s", fb)
	}
	for _, in := range []string{"package P;", "```sysml\npackage P;\n```", "Here you go:\n```\npackage P;\n```\nDone."} {
		if got := ExtractModel(in); got != "package P;" {
			t.Errorf("ExtractModel(%q) = %q", in, got)
		}
	}
}
