package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModelSourceConvertsAPIJSONAndReturnsWarnings(t *testing.T) {
	path := filepath.Join("..", "..", "..", "tests", "export", "testdata", "interchange", "library_identity.toolkit.full.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var warnings []string
	text, converted, err := ModelSource(path, data, func(message string) {
		warnings = append(warnings, message)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !converted {
		t.Fatal("API JSON was not marked converted")
	}
	if !strings.Contains(string(text), "library package DocumentLibrary") {
		t.Fatalf("converted source does not contain the document-defined library package:\n%s", text)
	}
	if len(warnings) == 0 {
		t.Fatal("expected a warning for the stale library identity")
	}
}

func TestModelSourceLeavesOtherFormatsUnchanged(t *testing.T) {
	data := []byte("not a model")
	text, converted, err := ModelSource("model.sysml", data, nil)
	if err != nil {
		t.Fatal(err)
	}
	if converted || string(text) != string(data) {
		t.Fatalf("ModelSource returned %q, converted=%t; want the original bytes", text, converted)
	}
}
