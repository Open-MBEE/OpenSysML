package reject

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/baseline"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/errata"
	oraclerepo "github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

// The committed baseline must state the pin, the bridges and the negative corpus
// its run measured, and they must still be this repository's. Needs no Java.
func TestCommittedBaselineStatesThisRepositorysProvenance(t *testing.T) {
	repo, current := currentProvenance(t)
	path := filepath.Join(repo, filepath.FromSlash(committedBaseline))
	if err := baseline.CheckCommitted(path, refreshCommand, current); err != nil {
		t.Fatal(err)
	}
}

// The corpus is ours, so a baseline recorded against a different one must fail
// as a movement to adjudicate, not as a provisioning defect.
func TestProvenanceGuardFailsOnAMovedCorpus(t *testing.T) {
	_, current := currentProvenance(t)
	corrupted := corruptBaseline(t, committedBaseline, current.Inputs[0].Digest, "sha256:"+strings.Repeat("0", 64))

	err := baseline.CheckCommitted(corrupted, refreshCommand, current)
	if err == nil {
		t.Fatal("a baseline recorded against another corpus was accepted")
	}
	for _, want := range []string{
		"provenance.inputs[negative-corpus].digest",
		current.Inputs[0].Digest,
		"material this repository owns has changed",
		refreshCommand,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the failure does not mention %q:\n%s", want, err)
		}
	}
}

// A baseline that states no provenance at all is the stale baseline this guard
// exists to catch, so it must fail rather than pass vacuously.
func TestProvenanceGuardFailsOnAnUnstatedProvenance(t *testing.T) {
	_, current := currentProvenance(t)
	corrupted := corruptBaseline(t, committedBaseline, `"recorded"`, `"recordedOn"`)

	err := baseline.CheckCommitted(corrupted, refreshCommand, current)
	if err == nil || !strings.Contains(err.Error(), "states no ISO recording date") {
		t.Fatalf("an undated baseline gave %v", err)
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
	overlay, err := errata.Load()
	if err != nil {
		t.Fatal(err)
	}
	corpusDir := "tools/referee/reject/testdata/negative"
	files, err := collectCases(repo, corpusDir)
	if err != nil {
		t.Fatal(err)
	}
	current, err := provenance(repo, "", corpusDir, overlay, files)
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
