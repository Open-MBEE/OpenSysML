package apijson

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

// The Java-free guard: the committed baseline must state the pin, the bridge
// source and the corpora as this checkout has them, so a re-pin or an edit under
// a root is caught without running the pilot.
func TestCommittedBaselineStatesThisRepositorysProvenance(t *testing.T) {
	path, current := currentProvenance(t)
	if err := baseline.CheckCommitted(path, refreshCommand, current); err != nil {
		t.Fatal(err)
	}
}

func TestProvenanceGuardFailsOnAMovedPin(t *testing.T) {
	path, current := currentProvenance(t)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	corrupted := filepath.Join(t.TempDir(), "baseline.json")
	moved := strings.Replace(string(content), current.PilotCommit, strings.Repeat("0", len(current.PilotCommit)), 1)
	if err := os.WriteFile(corrupted, []byte(moved), 0o600); err != nil {
		t.Fatal(err)
	}
	err = baseline.CheckCommitted(corrupted, refreshCommand, current)
	if err == nil || !strings.Contains(err.Error(), "pilotCommit") || !strings.Contains(err.Error(), refreshCommand) {
		t.Fatalf("want a pilotCommit mismatch naming the refresh command, got %v", err)
	}
}

func currentProvenance(t *testing.T) (string, baseline.Record) {
	t.Helper()
	root, err := repo.Choose("")
	if err != nil {
		t.Fatal(err)
	}
	current, err := provenance(root)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, filepath.FromSlash(committedBaseline)), current
}
