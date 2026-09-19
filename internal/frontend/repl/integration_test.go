package repl

import (
	"io"
	"strings"
	"testing"
)

type scriptReader struct {
	lines []string
	i     int
}

func (r *scriptReader) ReadLine(prompt string) (string, error) {
	if r.i >= len(r.lines) {
		return "", io.EOF
	}
	l := r.lines[r.i]
	r.i++
	return l, nil
}

func TestLoopEndToEnd(t *testing.T) {
	script := []string{
		"package P {",        // continuation begins
		"}",                  // closes → submits "package P {\n}"
		"namespace N;",       // second submission
		"%list",              // meta: should show both P and N
		"package P { }",      // redefine P (replaces prior)
		"import Missing::X;", // unresolved → diagnostic with caret
	}
	var out strings.Builder
	if err := Loop(&scriptReader{lines: script}, &out, NewSession()); err != nil {
		t.Fatalf("Loop error: %v", err)
	}
	got := out.String()

	// Continuation + summary for P.
	if !strings.Contains(got, "package P") {
		t.Errorf("missing package P summary:\n%s", got)
	}
	// Namespace N summary.
	if !strings.Contains(got, "namespace N") {
		t.Errorf("missing namespace N summary:\n%s", got)
	}
	// %list output shows accumulated declarations.
	if !strings.Contains(got, "package P") || !strings.Contains(got, "namespace N") {
		t.Errorf("%%list did not show session:\n%s", got)
	}
	// Unresolved import produces a diagnostic with a caret.
	if !strings.Contains(got, "^") {
		t.Errorf("missing caret diagnostic for unresolved import:\n%s", got)
	}
}

// TestActionDebuggerCommands tests %action command error handling.
func TestActionDebuggerCommands(t *testing.T) {
	script := []string{
		"%action NonExistent",
		"%step",
		"%tokens",
		"%continue",
	}
	var out strings.Builder
	if err := Loop(&scriptReader{lines: script}, &out, NewSession()); err != nil {
		t.Fatalf("Loop error: %v", err)
	}
	got := out.String()

	// Check error for non-existent action (empty session gives "no document loaded")
	if !strings.Contains(got, "error:") {
		t.Errorf("missing error:\n%s", got)
	}

	// Check errors for commands without active session
	if !strings.Contains(got, "no active action session") {
		t.Errorf("missing session error:\n%s", got)
	}
}

// TestStateMachineDebuggerCommands tests %state command error handling.
func TestStateMachineDebuggerCommands(t *testing.T) {
	script := []string{
		"%state NonExistent",
		"%current",
		"%events",
		"%advance 1",
	}
	var out strings.Builder
	if err := Loop(&scriptReader{lines: script}, &out, NewSession()); err != nil {
		t.Fatalf("Loop error: %v", err)
	}
	got := out.String()

	// Check error for non-existent state machine (empty session gives "no document loaded")
	if !strings.Contains(got, "error:") {
		t.Errorf("missing error:\n%s", got)
	}

	// Check errors for commands without active session
	if !strings.Contains(got, "no active state machine session") {
		t.Errorf("missing session error:\n%s", got)
	}
}

// TestBreakpointCommand tests %break command error handling.
func TestBreakpointCommand(t *testing.T) {
	script := []string{
		"%break middle",
	}
	var out strings.Builder
	if err := Loop(&scriptReader{lines: script}, &out, NewSession()); err != nil {
		t.Fatalf("Loop error: %v", err)
	}
	got := out.String()

	// Check error without active session
	if !strings.Contains(got, "no active action session") {
		t.Errorf("missing session error:\n%s", got)
	}
}

// TestStopCommand tests %stop command.
func TestStopCommand(t *testing.T) {
	script := []string{
		"%stop",
	}
	var out strings.Builder
	if err := Loop(&scriptReader{lines: script}, &out, NewSession()); err != nil {
		t.Fatalf("Loop error: %v", err)
	}
	got := out.String()

	// Check error without active session
	if !strings.Contains(got, "no active debugging session") {
		t.Errorf("missing session error:\n%s", got)
	}
}
