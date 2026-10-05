package diff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
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
// the pin rather than from a provisioned validator, over the library directory
// a run would hand the reference.
func currentProvenance(t *testing.T) (string, baseline.Record) {
	t.Helper()
	repo, err := oraclerepo.Root()
	if err != nil {
		t.Fatal(err)
	}
	libraries, err := librariesHandedOver(repo)
	if err != nil {
		t.Fatal(err)
	}
	current, err := provenance(repo, "", relativeTo(repo, libraries))
	if err != nil {
		t.Fatal(err)
	}
	return repo, current
}

// The Java-free guard digests the library directory a run would hand the
// reference, so an OPENSYSML_LIBRARY_PATH override inside the repository moves
// the recorded opensysml-libraries input instead of leaving the bundled digest.
func TestCurrentProvenanceFollowsTheLibraryOverride(t *testing.T) {
	clearLibraryOverride(t)
	repo, bundled := currentProvenance(t)
	bundledInput := libraryInput(t, bundled)

	if err := os.MkdirAll(filepath.Join(repo, "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(filepath.Join(repo, "build"), "pilot-diff-libraries-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	libraries := filepath.Join(root, openSysMLLibraries)
	if err := os.Mkdir(libraries, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libraries, "Ext.sysml"), []byte("library package Ext;\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv(libs.LibraryPathEnvVar, root)
	_, overridden := currentProvenance(t)
	input := libraryInput(t, overridden)
	if want := relativeTo(repo, libraries); input.Dir != want {
		t.Errorf("libraries input dir = %q, want the override %q", input.Dir, want)
	}
	if input.Files != 1 || input.Digest == bundledInput.Digest {
		t.Errorf("libraries input = %+v, want one file with a digest other than the bundled %s", input, bundledInput.Digest)
	}
}

// libraryInput is the opensysml-libraries input of a provenance record.
func libraryInput(t *testing.T, record baseline.Record) baseline.Input {
	t.Helper()
	for _, input := range record.Inputs {
		if input.Name == librariesInput {
			return input
		}
	}
	t.Fatalf("provenance records no %q input:\n%+v", librariesInput, record.Inputs)
	return baseline.Input{}
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
