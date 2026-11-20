//go:build !sysml_re && !sysml_nofmi

package engines

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
)

// fmiLinked reports whether this build links the FMI engine.
const fmiLinked = true

// A manifest tool named fmi, or an engine named smt, clashes with a built-in:
// the registration is refused as a DuplicateEngineError, not panicked over.
func TestDefaultFromEnvRefusesAManifestNamedLikeTheBuild(t *testing.T) {
	dir := t.TempDir()
	entry := analysis.ToolEntry{ToolName: "fmi", Version: "9", Executable: filepath.Join(dir, "fmi"), Variables: []string{"x"}}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fmi"+analysis.ManifestExt), data, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(analysis.ToolsEnv, dir)
	if _, err := DefaultFromEnv(); !errors.Is(err, analysis.ErrDuplicateEngine) {
		t.Fatalf("DefaultFromEnv with a manifest tool named fmi: %v, want DuplicateEngineError", err)
	}
}
