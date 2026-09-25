package analysis

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

// An object reply read from a file names its source, rendered like argv.
func TestAnObjectReplysFileSourceJoinsThePreview(t *testing.T) {
	entry := ToolEntry{ToolName: "Solver", Executable: standin(t),
		Variables:  []string{"mass", "tMax"},
		Invocation: &Invocation{Args: []string{"--out", "{outputDir}"}, Stdin: StdinSpec{Format: StdinNone}},
		Reply:      &Reply{Format: ReplyObject, Source: "file:{outputDir}/answer.json"}}
	r := registered(t, NewTool(entry))
	_, err := r.DryRunner(Auto()).RunTool(&runtime.ToolCall{ToolName: "Solver"})
	var dry *ToolDryRunError
	if !errors.As(err, &dry) {
		t.Fatalf("RunTool = %v, want a ToolDryRunError", err)
	}
	want := "reply: object from file:<outputDir>/answer.json — the protocol's JSON object, one key per output variable"
	if !strings.Contains(strings.Join(dry.Preview.Lines(), "\n"), want) {
		t.Errorf("the preview does not name the file the object is read from, want %q:\n%s", want, strings.Join(dry.Preview.Lines(), "\n"))
	}
}

// A higher-ranked engine may answer the call first; the preview reports it as
// undecided rather than previewing the manifest tool anyway.
func TestADryRunStopsAtAnEngineAheadOfTheTool(t *testing.T) {
	entry := ToolEntry{ToolName: "Solver", Executable: standin(t), Variables: []string{"mass", "tMax"}}
	r := registered(t,
		fakeEngine{name: "eager", kinds: []Kind{Compute}, authority: Proved},
		NewTool(entry))
	_, err := r.DryRunner(Auto()).RunTool(&runtime.ToolCall{ToolName: "Solver"})
	var undecided *PreviewUndecidedError
	if !errors.As(err, &undecided) {
		t.Fatalf("RunTool under auto = %v, want PreviewUndecidedError", err)
	}
	if undecided.Engine != "eager" || undecided.Tool != "Solver" {
		t.Errorf("undecided = %+v, want engine %q ahead of tool %q", undecided, "eager", "Solver")
	}
	_, err = r.DryRunner(Only(ToolEngineName("Solver"))).RunTool(&runtime.ToolCall{ToolName: "Solver"})
	var dry *ToolDryRunError
	if !errors.As(err, &dry) {
		t.Fatalf("RunTool under %q = %v, want ToolDryRunError", ToolEngineName("Solver"), err)
	}
	if dry.Preview.Tool != "Solver" {
		t.Errorf("the preview names %q, want Solver", dry.Preview.Tool)
	}
}
