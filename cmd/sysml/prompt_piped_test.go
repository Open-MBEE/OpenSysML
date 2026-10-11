package main

import (
	"strings"
	"testing"
)

// TestPipedLinesKeepTabs checks that lines piped into the prompt are taken byte
// for byte: a TAB is indentation, string content or comment text there, never a
// completion keystroke, so the session ends as the lines say and exits clean.
func TestPipedLinesKeepTabs(t *testing.T) {
	binary := buildCLI(t)

	model := "package P {\n" +
		"\tpart def A;\n" +
		"\tattribute s : ScalarValues::String = \"a\tb\";\n" +
		"\t// a\tcomment\n" +
		"}\n" +
		"%list\n" +
		"P::s\n"
	got := runStdin(t, binary, model)

	if got.status != exitHolds {
		t.Errorf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
	}
	if strings.Contains(got.output(), "panic:") {
		t.Errorf("the prompt panicked:\n%s", got.output())
	}
	if got.stderr != "" {
		t.Errorf("stderr is not empty:\n%s", got.output())
	}
	for _, want := range []string{
		"✓ package P\n",
		"\tpart def A;\n",
		"\tattribute s : ScalarValues::String = \"a\tb\";\n",
		"\t// a\tcomment\n",
		"✓ P::s\n",
		"= \"a\\tb\"\n",
	} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("stdout is missing %q:\n%s", want, got.output())
		}
	}
	if strings.Contains(got.stdout, "sysml> ") {
		t.Errorf("stdout echoes a prompt to a pipe:\n%s", got.output())
	}
}
