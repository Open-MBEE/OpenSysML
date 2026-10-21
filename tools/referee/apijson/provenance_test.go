package apijson

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
	"github.com/Open-MBEE/OpenSysML/tools/referee/diff"
)

// The Java-free guard: the committed baseline must state the pin, the bridge and
// writer sources and the corpora as this checkout has them, without the pilot.
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

// The bridge reads this repository's own libraries after the pinned standard
// library; the record states them as an input, so an edit under them is a
// mismatch like an edit under a corpus root.
func TestProvenanceStatesTheLibrariesTheBridgeReads(t *testing.T) {
	_, current := currentProvenance(t)
	var libraries *baseline.Input
	for i := range current.Inputs {
		if current.Inputs[i].Name == diff.LibrariesInput {
			libraries = &current.Inputs[i]
		}
	}
	if libraries == nil || libraries.Dir != "internal/workspace/libs/stdlib/OpenSysML Libraries" || libraries.Origin != baseline.OriginOurs || libraries.Files == 0 || libraries.Digest == "" {
		t.Fatalf("want the %q input at the bundled libraries with files and a digest, got %+v", diff.LibrariesInput, current.Inputs)
	}
}

func currentProvenance(t *testing.T) (string, baseline.Record) {
	t.Helper()
	root, err := repo.Choose("")
	if err != nil {
		t.Fatal(err)
	}
	libraries, err := diff.LibrariesHandedOver(root)
	if err != nil {
		t.Fatal(err)
	}
	// The test binary is compiled from the checkout it runs in: root is both.
	current, err := provenance(root, root, libraries)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, filepath.FromSlash(committedBaseline)), current
}

// Editing the writer package moves its digest, and the guard names that tool
// rather than letting the movement surface as bare count drift.
func TestProvenanceGuardFailsOnAnEditedWriter(t *testing.T) {
	path, current := currentProvenance(t)
	var writer *baseline.Tool
	for i := range current.Tools {
		if current.Tools[i].Name == writerName {
			writer = &current.Tools[i]
		}
	}
	if writer == nil || writer.Source != writerSource || writer.Release != "" {
		t.Fatalf("want a release-less %s tool digesting %s, got %+v", writerName, writerSource, current.Tools)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	corrupted := filepath.Join(t.TempDir(), "baseline.json")
	stale := strings.Replace(string(content), writer.SourceDigest, "sha256:"+strings.Repeat("0", 64), 1)
	if stale == string(content) {
		t.Fatalf("committed baseline does not record the writer digest %s", writer.SourceDigest)
	}
	if err := os.WriteFile(corrupted, []byte(stale), 0o600); err != nil {
		t.Fatal(err)
	}
	err = baseline.CheckCommitted(corrupted, refreshCommand, current)
	want := "tools[" + writerName + "].sourceDigest"
	if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), refreshCommand) {
		t.Fatalf("want a %s mismatch naming the refresh command, got %v", want, err)
	}
}
