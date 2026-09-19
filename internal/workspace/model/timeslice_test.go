package model

import (
	"os"
	"testing"
)

func TestTimeSliceExample(t *testing.T) {
	path := "../../../examples/sysml-v2-training/27. Occurrences/Time Slice and Snapshot Example.sysml"

	// Skip if training examples not downloaded
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("Training examples not downloaded (run ./scripts/download-training-examples.sh)")
	}

	ws := NewWorkspace()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	ws.Open("test.sysml", data, 1)
	diags := ws.Diagnostics("test.sysml")

	t.Logf("Found %d diagnostics", len(diags))
	for _, d := range diags {
		t.Logf("  %s", d.Message)
	}
}
