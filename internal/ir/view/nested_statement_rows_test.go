package view

import (
	"strings"
	"testing"
)

// A loop or conditional nested as a statement in a textual body (§8.2.3.17
// action-body-textual) lists its own body's statements under its head, indented,
// rather than its head alone.
func TestNestedStatementsListTheirBodies(t *testing.T) {
	rendering := renderSource(t, "Diagrams::flow", `package Model {
	private import ScalarValues::*;
	action def Monitor {
		attribute count : Integer;
		attribute flag : Boolean;
		action wait while count < 10 {
			assign count := count + 1;
			if count > 5 {
				assign flag := true;
			} else {
				assign flag := false;
			}
			loop {
				assign count := count + 2;
			} until count > 20;
			loop {
				assign count := 0;
			}
		}
	}
}
package Diagrams {
	private import StandardViewDefinitions::*;
	view flow : ActionFlowView {
		expose Model::Monitor;
	}
}`)
	text := rendering.Text()
	for _, want := range []string{
		"assign count := count + 1",
		"if count > 5",
		"  assign flag := true",
		"else",
		"  assign flag := false",
		"loop",
		"  assign count := count + 2",
		"until count > 20",
	} {
		if !strings.Contains(text, want+"\n") && !strings.Contains(text, want+" ") {
			t.Errorf("text form lacks the row %q:\n%s", want, text)
		}
	}
	if got := strings.Count(text, "until"); got != 1 {
		t.Errorf("a `loop { }` written without `until` gets no until row; %d until rows in:\n%s", got, text)
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if !strings.Contains(dot, "\u00a0\u00a0assign flag := true") {
		t.Errorf("DOT keeps the nested row's indent as no-break spaces:\n%s", dot)
	}
}
