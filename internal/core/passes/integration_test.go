package passes

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

var updateGolden = flag.Bool("update", false, "update golden files")

func runPassesGolden(t *testing.T, name string) {
	t.Helper()
	srcPath := filepath.Join("..", "..", "..", "testdata", "passes", name+".sysml")
	data, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	sf := source.New(name+".sysml", data)
	root, parseDiags, idx := analyzeInputs(t, name+".sysml", string(data))
	diags := Analyze(name+".sysml", root, parseDiags, idx)

	var b strings.Builder
	if len(diags) == 0 {
		b.WriteString("(no diagnostics)\n")
	} else {
		lines := sf.Lines()
		for _, d := range diags {
			pos := lines.PosAt(d.Span.Offset)
			fmt.Fprintf(&b, "%d:%d %s [%s/%s] %s\n", pos.Line, pos.Col, d.Severity, d.Source, d.Code, d.Message)
		}
	}
	got := b.String()

	goldenPath := filepath.Join("..", "..", "..", "testdata", "passes", name+".golden")
	if *updateGolden {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if got != string(want) {
		t.Fatalf("golden mismatch for %s:\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestPassesGoldenClean(t *testing.T)       { runPassesGolden(t, "clean") }
func TestPassesGoldenErrors(t *testing.T)      { runPassesGolden(t, "errors") }
func TestPassesGoldenConstraints(t *testing.T) { runPassesGolden(t, "constraints") }

// A bare `import` is non-conforming notation the reference rejects: it errors at
// every nesting depth (D2).
func TestPassesGoldenImportNoVisibility(t *testing.T) {
	runPassesGolden(t, "import_no_visibility")
}

// The corpus notation reports only the two duplicate-name warnings the pinned
// validator reports on the same fixture, at the same positions (matched run,
// w6c): a regression in the conjugated end or portion prefix would add more.
func TestPassesGoldenCorpusNotation(t *testing.T) { runPassesGolden(t, "corpus_notation") }

// A multi-valued feature is unique unless declared nonunique: a const-decidable
// repeat is a diagnostic, one only a run decides is left to the runtime.
func TestPassesGoldenUniqueValues(t *testing.T) { runPassesGolden(t, "unique_values") }
