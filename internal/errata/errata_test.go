package errata

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	liberrata "github.com/Open-MBEE/OpenSysML/internal/core/libs/errata"
)

const repoRoot = "../.."

// requireEnv turns an absent corpus into a failure, as CI does for the gates.
const requireEnv = "OPENSYSML_REQUIRE_PILOT_CORPORA"

func TestRegistryIsAccepted(t *testing.T) {
	overlay, err := Load()
	if err != nil {
		t.Fatalf("load the declared registry: %v", err)
	}
	if len(overlay.Entries()) == 0 {
		t.Fatal("the registry declares no entry, so nothing it claims can be checked")
	}
}

// TestAsPublishedMatchesTheCorpus is what stops an entry from rotting when the
// corpus is re-vendored: every entry must still match the bytes on disk.
func TestAsPublishedMatchesTheCorpus(t *testing.T) {
	overlay, err := Load()
	if err != nil {
		t.Fatalf("load the declared registry: %v", err)
	}
	var missing []string
	for _, entry := range overlay.Entries() {
		content, readErr := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(entry.Path))) // #nosec G304 -- the path comes from the declared registry
		if readErr != nil {
			if os.IsNotExist(readErr) {
				missing = append(missing, entry.Path)
				continue
			}
			t.Fatalf("read %s: %v", entry.Path, readErr)
		}
		if _, err := liberrata.Apply(entry, content); err != nil {
			t.Fatalf("an entry no longer matches the published corpus: %v", err)
		}
	}
	sort.Strings(missing)
	if len(missing) == 0 {
		return
	}
	if os.Getenv(requireEnv) != "" {
		t.Fatalf("%s is set and these entries' files are absent: %s", requireEnv, strings.Join(missing, ", "))
	}
	t.Skipf("pilot corpora not downloaded (run ./scripts/download-pilot-corpora.sh); unchecked: %s", strings.Join(missing, ", "))
}

// TestEveryEntryIsDocumented ties each entry to its omg-issues.md row, quoting
// its citation and its published text there rather than in the registry alone.
func TestEveryEntryIsDocumented(t *testing.T) {
	page, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(IssuesPath)))
	if err != nil {
		t.Fatalf("read %s: %v", IssuesPath, err)
	}
	text := string(page)
	for _, entry := range Registry() {
		if !strings.Contains(text, "### "+entry.Heading) {
			t.Errorf("%s: %s has no `### %s` section", IssuesPath, entry.ID, entry.Heading)
		}
		if !strings.Contains(text, entry.Citation) {
			t.Errorf("%s: %s does not quote the citation %s", IssuesPath, entry.ID, entry.Citation)
		}
		if !strings.Contains(text, strings.TrimSpace(entry.AsPublished)) {
			t.Errorf("%s: %s does not quote the published text", IssuesPath, entry.ID)
		}
		if entry.Corrects() && !strings.Contains(text, strings.TrimSpace(entry.Corrected)) {
			t.Errorf("%s: %s does not quote the correction", IssuesPath, entry.ID)
		}
	}
}

func TestUnderKeysCorrectionsByCorpusPath(t *testing.T) {
	overlay, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	under := overlay.Under("examples/pilot-corpora/sysml-examples")
	if _, ok := under["Geometry Examples/VehicleGeometryAndCoordinateFrames.sysml"]; !ok {
		t.Fatalf("the geometry correction is not keyed under the sysml-examples root: %v", under)
	}
	if len(overlay.Under("cmd/pilot-reject/testdata/negative")) != 0 {
		t.Fatal("a correction was claimed for our own negative corpus")
	}
}

// TestMaterializeLeavesThePublishedCorpusByteIdentical is the immutability
// contract: applying the overlay writes only into the copy.
func TestMaterializeLeavesThePublishedCorpusByteIdentical(t *testing.T) {
	const dir = "examples/pilot-corpora/sysml-examples"
	root := filepath.Join(repoRoot, filepath.FromSlash(dir))
	if _, err := os.Stat(root); err != nil {
		if os.Getenv(requireEnv) != "" {
			t.Fatalf("%s is set and %s is absent", requireEnv, dir)
		}
		t.Skip("pilot corpora not downloaded (run ./scripts/download-pilot-corpora.sh)")
	}
	overlay, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	before := hashTree(t, root)
	dst := filepath.Join(t.TempDir(), "corrected")
	applied, err := Materialize(overlay, repoRoot, dir, dst)
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if len(applied) == 0 {
		t.Fatal("nothing was applied")
	}
	if after := hashTree(t, root); after != before {
		t.Fatalf("the published corpus changed: %s -> %s", before, after)
	}
	for _, entry := range applied {
		rel := strings.TrimPrefix(entry.Path, dir+"/")
		content, err := os.ReadFile(filepath.Join(dst, filepath.FromSlash(rel))) // #nosec G304 -- the path is inside the copy the test just made
		if err != nil {
			t.Fatalf("read the copy of %s: %v", rel, err)
		}
		if strings.Contains(string(content), entry.AsPublished) {
			t.Fatalf("%s still reads as published in the copy", rel)
		}
		if !strings.Contains(string(content), entry.Corrected) {
			t.Fatalf("%s does not read as corrected in the copy", rel)
		}
	}
}

// TestMaterializeVerifiesDocumentedEntries: a documented-only entry whose line
// no longer reads as published fails the materialization, even in a file (or a
// corpus) whose corrections all still apply; only the corrections are reported applied.
func TestMaterializeVerifiesDocumentedEntries(t *testing.T) {
	const dir = "examples/pilot-corpora/x"
	corrected := Entry{
		ID: "T1", Heading: "a test entry", Path: dir + "/Sample.sysml", Line: 1,
		AsPublished: "a", Corrected: "A", Citation: "SysML v2 §9.8.9.1", Derivation: "test entry.",
	}
	documented := Entry{
		ID: "T2", Heading: "a test entry", Path: dir + "/Other.sysml", Line: 2,
		AsPublished: "y", Citation: "SysML v2 §9.8.9.1", Derivation: "no intended reading.",
	}
	overlay, err := liberrata.New(Roots, []Entry{corrected, documented})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	materialize := func(other string) (string, []Entry, error) {
		t.Helper()
		repo := t.TempDir()
		root := filepath.Join(repo, filepath.FromSlash(dir))
		if err := os.MkdirAll(root, 0o750); err != nil {
			t.Fatal(err)
		}
		for name, content := range map[string]string{"Sample.sysml": "a\nb\n", "Other.sysml": other} {
			if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		out := filepath.Join(repo, "out")
		applied, err := Materialize(overlay, repo, dir, filepath.Join(out, "corrected"))
		return out, applied, err
	}
	out, applied, err := materialize("x\ny\n")
	if err != nil {
		t.Fatalf("materialize over a corpus as published: %v", err)
	}
	if len(applied) != 1 || applied[0].ID != "T1" {
		t.Fatalf("applied = %v, want the one correction", applied)
	}
	if names := dirNames(t, out); !slices.Equal(names, []string{"corrected"}) {
		t.Fatalf("after a materialization: %v, want the corrected copy alone", names)
	}
	out, _, err = materialize("x\nother\n")
	if err == nil {
		t.Fatal("a corpus whose documented-only line rotted was materialized")
	}
	if _, err := os.Stat(out); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("after a failed materialization: %v, want the directory it created gone with the partial copy", dirNames(t, out))
	}
}

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func hashTree(t *testing.T, root string) string {
	t.Helper()
	sum := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, err := os.ReadFile(path) // #nosec G304 -- the path comes from walking the corpus root
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		sum.Write([]byte(filepath.ToSlash(rel)))
		sum.Write(content)
		return nil
	})
	if err != nil {
		t.Fatalf("hash %s: %v", root, err)
	}
	return hex.EncodeToString(sum.Sum(nil))
}

// TestEntryOutsideThePublishedRootsIsRejected: a correction to our own material is
// a defect to fix, not an erratum to declare.
func TestEntryOutsideThePublishedRootsIsRejected(t *testing.T) {
	entry := Registry()[0]
	entry.Path = "testdata/passes/constraints.sysml"
	if _, err := liberrata.New(Roots, []Entry{entry}); err == nil {
		t.Fatal("a correction to our own material was accepted")
	}
}

// TestRegistryEndsWithTheLibraryEntries pins the report order: the corpus entries first,
// then the bundled library's exactly as the product declares them.
func TestRegistryEndsWithTheLibraryEntries(t *testing.T) {
	registry, library := Registry(), liberrata.LibraryEntries()
	if len(registry) <= len(library) {
		t.Fatalf("the registry holds %d entries, the library alone %d", len(registry), len(library))
	}
	if got := registry[len(registry)-len(library):]; !slices.Equal(got, library) {
		t.Fatalf("the registry's tail differs from the library entries:\n%v\n%v", got, library)
	}
}
