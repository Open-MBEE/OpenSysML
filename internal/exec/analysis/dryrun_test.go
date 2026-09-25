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
