package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tests/testutil/gobuild"
)

// A multi-valued tool output runs through -record-run into the Records package
// and into the document the model tabulates, the markdown cell holding every
// item the CSV reply's records became.
func TestToolSequenceRecordedIntoADocument(t *testing.T) {
	binary := buildCLI(t)

	// The CSV-answering stand-in, built once per test binary.
	standin := filepath.Join(t.TempDir(), "toolreply")
	build := exec.Command("go", gobuild.Args(standin)...)
	build.Dir = filepath.Join("..", "..", "internal", "exec", "analysis", "testdata", "toolreply")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the reply stand-in: %v\n%s", err, out)
	}

	manifest := filepath.Join(t.TempDir(), "thermal.json")
	entry := fmt.Sprintf(`{
  "toolName": "Thermal",
  "executable": %q,
  "variables": ["temps"],
  "invocation": {"args": ["csv-all"]},
  "reply": {"format": "csv", "outputs": {"temps": {"column": "T", "row": "all", "unitColumn": "U"}}}
}`, standin)
	if err := os.WriteFile(manifest, []byte(entry), 0o644); err != nil {
		t.Fatal(err)
	}

	// -record-run edits the file it records; give it a copy of its own.
	src, err := os.ReadFile(filepath.Join("testdata", "tool_sequence_record.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(t.TempDir(), "model.sysml")
	if err := os.WriteFile(work, src, 0o644); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "report.md")
	render := exec.Command(binary, work, "-record-run", "Profiles::Case",
		"-render-document", "Reporting::Doc", "-o", out)
	render.Env = append(os.Environ(), "OPENSYSML_TOOLS="+filepath.Dir(manifest))
	if output, err := render.CombinedOutput(); err != nil {
		t.Fatalf("record and render: %v\n%s", err, output)
	}
	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	text := string(written)
	for _, want := range []string{"300, 310.5, 341.2", "temps"} {
		if !strings.Contains(text, want) {
			t.Errorf("document is missing %q:\n%s", want, text)
		}
	}
}
