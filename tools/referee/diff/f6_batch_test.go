package diff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// F6: a root is one invocation of the reference, whatever its files are named,
// so nothing depends on unique base names or an import ordering any more.
func TestPilotDiagnosticsIsOneBatchPerRoot(t *testing.T) {
	repo := t.TempDir()
	files := []string{"a/Model.sysml", "b/Model.sysml", "c/Other.sysml"}
	for _, rel := range files {
		path := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("package P;\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	log := filepath.Join(repo, "args.txt")
	validator := filepath.Join(repo, "validate-sysml-batch")
	if err := os.WriteFile(validator, []byte(stubValidator(log,
		"b/Model.sysml:4:1: warning: Duplicate of other owned member name")), 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := pilotDiagnostics(validator, repo, ".", files, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got["b/Model.sysml"]) != 1 || got["b/Model.sysml"][0].Line != 4 {
		t.Errorf("diagnostics = %+v", got)
	}

	args := readFile(t, log)
	if n := strings.Count(args, "--root"); n != 1 {
		t.Errorf("the validator ran %d times; args:\n%s", n, args)
	}
	for _, rel := range files {
		if !strings.Contains(args, filepath.FromSlash(rel)) {
			t.Errorf("%s was not part of the batch; args:\n%s", rel, args)
		}
	}
}
