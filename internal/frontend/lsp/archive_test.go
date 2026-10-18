package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
)

// sysandFixture is the committed project laid out as sysand installs one: its
// .project.json uses Requirements Management, installed as a directory under
// .sysand/lib, which in turn uses sysmod, installed there as an archive.
func sysandFixture(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "tests", "kpar", "testdata", "sysand"))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestWorkspaceIndexesTheMembersOfAnInstalledArchive(t *testing.T) {
	proj := sysandFixture(t)
	s, fc := projectWorkspace(t, proj, nil)
	archive := filepath.Join(proj, ".sysand", "lib", "mbse4u-sysmod_5.1.4"+project.ArchiveExt)
	for _, member := range []string{"SYSMOD.sysml", "SYSMOD4AI.sysml", "SYSMODRequirementsBoilerPlates.sysml"} {
		if s.ws.Document(project.MemberPath(archive, member)) == nil {
			t.Errorf("%s not indexed from the installed archive", member)
		}
	}
	if s.ws.Document(project.MemberPath(archive, "README.md")) != nil {
		t.Error("the archive's README was indexed as a model")
	}
	library := filepath.Join(proj, ".sysand", "lib", "ode4hera-requirements-management_2.0.1", "RequirementsManagement.sysml")
	if s.ws.Document(library) == nil {
		t.Fatalf("%s not indexed from the installed directory", library)
	}
	// The directory's library file resolves against the archive's packages.
	openFile(t, s, library, readFixture(t, library))
	if msgs := diagnosticsFor(fc, library); len(msgs) != 0 {
		t.Errorf("diagnostics for the installed library = %v, want none with sysmod read from its archive", msgs)
	}
}

// A malformed archive under a library root is passed over; the projects
// beside it are still indexed.
func TestWorkspaceSkipsAMalformedArchiveUnderALibraryRoot(t *testing.T) {
	proj, store := interchangeFixture(t)
	broken := t.TempDir()
	if err := os.WriteFile(filepath.Join(broken, "broken"+project.ArchiveExt), []byte("not a zip"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, _ := projectWorkspace(t, proj, map[string]any{"sysml": map[string]any{"libraryRoots": []any{broken, store}}})
	vendored := filepath.Join(store, "acme", "Vendored Parts", "VendoredParts.sysml")
	if s.ws.Document(vendored) == nil {
		t.Fatalf("%s not indexed with a malformed archive under another root", vendored)
	}
	for _, name := range s.ws.DocumentNames() {
		if strings.HasPrefix(name, broken+string(filepath.Separator)) {
			t.Errorf("%s indexed from the malformed archive", name)
		}
	}
}
