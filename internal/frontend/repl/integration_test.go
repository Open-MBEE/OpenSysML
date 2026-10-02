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

// interruptReader yields scripted lines, a nil-error entry for each line and
// ErrInterrupt where interrupted is set.
type interruptReader struct {
	lines       []string
	interrupted []bool
	prompts     []string
}

func (r *interruptReader) ReadLine(prompt string) (string, error) {
	r.prompts = append(r.prompts, prompt)
	i := len(r.prompts) - 1
	if i >= len(r.lines) {
		return "", io.EOF
	}
	if r.interrupted[i] {
		return r.lines[i], ErrInterrupt
	}
	return r.lines[i], nil
}

// TestLoopInterrupt checks that Ctrl-C discards a typed line and a buffered
// continuation, and ends the session at an empty primary prompt.
func TestLoopInterrupt(t *testing.T) {
	r := &interruptReader{
		lines:       []string{"package Typed", "package Cont {", "part def X;", "", "package Kept;", "", "package Never;"},
		interrupted: []bool{true, false, false, true, false, true, false},
	}
	var out strings.Builder
	s := NewSession()
	if err := Loop(r, &out, s); err != nil {
		t.Fatalf("Loop error: %v", err)
	}
	wantPrompts := []string{primaryPrompt, primaryPrompt, contPrompt, contPrompt, primaryPrompt, primaryPrompt}
	if strings.Join(r.prompts, "|") != strings.Join(wantPrompts, "|") {
		t.Errorf("prompts = %q, want %q", r.prompts, wantPrompts)
	}
	got := out.String()
	if !strings.Contains(got, "package Kept") {
		t.Errorf("submission after an interrupt was not taken:\n%s", got)
	}
	for _, gone := range []string{"Typed", "Cont", "Never"} {
		if strings.Contains(got, gone) {
			t.Errorf("output mentions %q, which an interrupt should have discarded or the exit never read:\n%s", gone, got)
		}
	}
}
