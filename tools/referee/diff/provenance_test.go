package diff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/baseline"
	oraclerepo "github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

// The committed baseline must state the pin, the bridges and the corpora its run
// measured, and they must still be this repository's. Needs no Java validator.
func TestCommittedBaselineStatesThisRepositorysProvenance(t *testing.T) {
	repo, current := currentProvenance(t)
	path := filepath.Join(repo, filepath.FromSlash(committedBaseline))
	if err := baseline.CheckCommitted(path, refreshCommand, current); err != nil {
		t.Fatal(err)
	}
}

// A baseline recorded against another pin must fail naming the field, both
// values and the refresh command, or the guard tells a reader nothing.
func TestProvenanceGuardFailsOnAMovedPin(t *testing.T) {
	_, current := currentProvenance(t)
	corrupted := corruptBaseline(t, committedBaseline,
		`"pilotTag": "`+current.PilotTag+`"`, `"pilotTag": "2025-02"`)

	err := baseline.CheckCommitted(corrupted, refreshCommand, current)
	if err == nil {
		t.Fatal("a baseline recorded against another pin was accepted")
	}
	for _, want := range []string{"provenance.pilotTag", "2025-02", current.PilotTag, refreshCommand} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the failure does not mention %q:\n%s", want, err)
		}
	}
}

// currentProvenance is the repository's provenance as it stands, resolved from
// the pin rather than from a provisioned validator.
func currentProvenance(t *testing.T) (string, baseline.Record) {
	t.Helper()
	repo, err := oraclerepo.Root()
	if err != nil {
		t.Fatal(err)
	}
	current, err := provenance(repo, "")
	if err != nil {
		t.Fatal(err)
	}
	return repo, current
}

// corruptBaseline copies a committed baseline with one substitution applied, so
// the guard is exercised against a damaged field without touching the record.
func corruptBaseline(t *testing.T, rel, was, now string) string {
	t.Helper()
	repo, err := oraclerepo.Root()
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	corrupted := strings.Replace(string(content), was, now, 1)
	if corrupted == string(content) {
		t.Fatalf("%s contains no %s to corrupt", rel, was)
	}
	path := filepath.Join(t.TempDir(), filepath.Base(rel))
	if err := os.WriteFile(path, []byte(corrupted), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
