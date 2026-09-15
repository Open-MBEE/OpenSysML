package errata

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
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

// TestEntryWithoutProvenanceIsRejected keeps the provenance a mechanism rather
// than a convention: each shape below must be refused.
func TestEntryWithoutProvenanceIsRejected(t *testing.T) {
	valid := Entry{
		ID:          "F82",
		Heading:     "`x = 1` is dimensionless",
		Path:        "examples/pilot-corpora/sysml-examples/Sample.sysml",
		Line:        3,
		AsPublished: "    x = 1;",
		Corrected:   "    x = 2;",
		Citation:    "SysML v2 §9.8.9.1",
		Derivation:  "the published text adds a dimensionless value to a length.",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("the control entry must be accepted: %v", err)
	}

	tests := map[string]func(Entry) Entry{
		"no citation":         func(e Entry) Entry { e.Citation = ""; return e },
		"no derivation":       func(e Entry) Entry { e.Derivation = ""; return e },
		"no issues row":       func(e Entry) Entry { e.ID = ""; return e },
		"no issues section":   func(e Entry) Entry { e.Heading = ""; return e },
		"no file":             func(e Entry) Entry { e.Path = ""; return e },
		"no line":             func(e Entry) Entry { e.Line = 0; return e },
		"no published text":   func(e Entry) Entry { e.AsPublished = "   "; return e },
		"correction is a nop": func(e Entry) Entry { e.Corrected = e.AsPublished; return e },
		"multi-line span":     func(e Entry) Entry { e.Corrected = "a\nb"; return e },
		"our own material":    func(e Entry) Entry { e.Path = "testdata/passes/constraints.sysml"; return e },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			entry := mutate(valid)
			if err := entry.Validate(); err == nil {
				t.Fatalf("%s was accepted", name)
			}
			if _, err := New([]Entry{entry}); err == nil {
				t.Fatalf("%s was accepted into an overlay", name)
			}
		})
	}
}

func TestDuplicateEntriesAreRejected(t *testing.T) {
	entry := Registry()[0]
	if _, err := New([]Entry{entry, entry}); err == nil {
		t.Fatal("the same entry was accepted twice")
	}
	other := entry
	other.ID = entry.ID + "b"
	if _, err := New([]Entry{entry, other}); err == nil {
		t.Fatal("two entries covering one line were accepted")
	}
	other.Line++
	if _, err := New([]Entry{entry, other}); err != nil {
		t.Fatalf("two entries covering different lines of one file were refused: %v", err)
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
		if _, err := Apply(entry, content); err != nil {
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

func TestApplyRewritesOnlyTheDeclaredLine(t *testing.T) {
	entry := Entry{
		ID: "F00", Heading: "a test entry", Path: "examples/pilot-corpora/x/Sample.sysml", Line: 2,
		AsPublished: "b", Corrected: "B",
		Citation: "SysML v2 §9.8.9.1", Derivation: "test entry.",
	}
	got, err := Apply(entry, []byte("a\nb\nc\n"))
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if string(got) != "a\nB\nc\n" {
		t.Fatalf("apply rewrote %q", got)
	}
}

func TestApplyRefusesAnEntryThatNoLongerMatches(t *testing.T) {
	entry := Entry{
		ID: "F00", Heading: "a test entry", Path: "examples/pilot-corpora/x/Sample.sysml", Line: 2,
		AsPublished: "b", Corrected: "B",
		Citation: "SysML v2 §9.8.9.1", Derivation: "test entry.",
	}
	if _, err := Apply(entry, []byte("a\nother\nc\n")); err == nil {
		t.Fatal("a rotted entry was applied")
	}
	if _, err := Apply(entry, []byte("a\n")); err == nil {
		t.Fatal("an entry past the end of the file was applied")
	}
}

// TestDocumentedEntrySubstitutesNothing pins the documented-only shape: the
// defect is recorded, and the text the oracles read is the published one.
func TestDocumentedEntrySubstitutesNothing(t *testing.T) {
	entry := Entry{
		ID: "F00", Heading: "a test entry", Path: "examples/pilot-corpora/x/Sample.sysml", Line: 1,
		AsPublished: "a", Citation: "SysML v2 §9.8.9.1", Derivation: "no intended reading.",
	}
	overlay, err := New([]Entry{entry})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if len(overlay.Corrections()) != 0 || len(overlay.Documented()) != 1 {
		t.Fatalf("documented entry counted as a correction: %+v", overlay)
	}
	if under := overlay.Under("examples/pilot-corpora/x"); len(under) != 0 {
		t.Fatalf("documented entry offered for substitution: %v", under)
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
	applied, err := overlay.Materialize(repoRoot, dir, dst)
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

// TestApplyAllChecksEveryLineBeforeSubstituting keeps a file with several
// entries all-or-nothing: one rotted entry fails the file, and documented-only
// entries are verified but substitute nothing.
func TestApplyAllChecksEveryLineBeforeSubstituting(t *testing.T) {
	corrected := Entry{
		ID: "T1", Heading: "a test entry", Path: "examples/pilot-corpora/x/Sample.sysml", Line: 1,
		AsPublished: "a", Corrected: "A", Citation: "SysML v2 §9.8.9.1", Derivation: "test entry.",
	}
	documented := Entry{
		ID: "T2", Heading: "a test entry", Path: "examples/pilot-corpora/x/Sample.sysml", Line: 3,
		AsPublished: "c", Citation: "SysML v2 §9.8.9.1", Derivation: "no intended reading.",
	}
	got, err := ApplyAll([]Entry{corrected, documented}, []byte("a\nb\nc\n"))
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if string(got) != "A\nb\nc\n" {
		t.Fatalf("apply rewrote %q", got)
	}
	if _, err := ApplyAll([]Entry{corrected, documented}, []byte("a\nb\nother\n")); err == nil {
		t.Fatal("a file whose documented entry rotted was still corrected")
	}
	got, err = ApplyAll([]Entry{documented}, []byte("a\nb\nc\n"))
	if err != nil || string(got) != "a\nb\nc\n" {
		t.Fatalf("documented-only entries changed the text: %q, %v", got, err)
	}
}

// fakeSource serves fixed bytes under library-relative names.
type fakeSource map[string]string

func (s fakeSource) List() []string {
	names := make([]string, 0, len(s))
	for name := range s {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (s fakeSource) Read(name string) ([]byte, error) {
	content, ok := s[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return []byte(content), nil
}

// TestLibrarySourceCorrectsOnlyTheDeclaredFiles is the library-side contract:
// the published source is read, never written, files without an entry pass
// through byte-identical, a documented-only entry is verified but substitutes
// nothing, and a rotted entry of either kind fails the read.
func TestLibrarySourceCorrectsOnlyTheDeclaredFiles(t *testing.T) {
	entry := Entry{
		ID: "T1", Heading: "a test entry", Path: LibraryRoot + "/Lib/Units.sysml", Line: 2,
		AsPublished: "    attribute u = m^-2;", Corrected: "    attribute u = m^2;",
		Citation: "KerML 7.4.9", Derivation: "test entry.",
	}
	documented := Entry{
		ID: "T2", Heading: "a test entry", Path: LibraryRoot + "/Lib/Noted.sysml", Line: 2,
		AsPublished: "    attribute v = s^-2;", Citation: "KerML 7.4.9", Derivation: "no intended reading.",
	}
	overlay, err := New([]Entry{entry, documented})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	published := fakeSource{
		"Lib/Units.sysml": "package Units {\n    attribute u = m^-2;\n}\n",
		"Lib/Noted.sysml": "package Noted {\n    attribute v = s^-2;\n}\n",
		"Lib/Other.sysml": "package Other {}\n",
	}
	src := overlay.LibrarySource(published)
	if got := src.List(); !reflect.DeepEqual(got, published.List()) {
		t.Fatalf("the corrected source lists %v", got)
	}
	got, err := src.Read("Lib/Units.sysml")
	if err != nil || string(got) != "package Units {\n    attribute u = m^2;\n}\n" {
		t.Fatalf("corrected read: %q, %v", got, err)
	}
	if got, err := src.Read("Lib/Other.sysml"); err != nil || string(got) != published["Lib/Other.sysml"] {
		t.Fatalf("a file without an entry was changed: %q, %v", got, err)
	}
	if got, err := src.Read("Lib/Noted.sysml"); err != nil || string(got) != published["Lib/Noted.sysml"] {
		t.Fatalf("a documented-only entry changed the text: %q, %v", got, err)
	}
	if again, _ := published.Read("Lib/Units.sysml"); string(again) != published["Lib/Units.sysml"] {
		t.Fatal("the published source was written to")
	}
	if _, err := src.Read("Lib/Missing.sysml"); err == nil {
		t.Fatal("a file the published source lacks was served")
	}
	published["Lib/Units.sysml"] = "package Units {\n    attribute u = m^-3;\n}\n"
	if _, err := src.Read("Lib/Units.sysml"); err == nil {
		t.Fatal("a file whose declared line changed was served uncorrected")
	}
	published["Lib/Noted.sysml"] = "package Noted {\n    attribute v = s^-3;\n}\n"
	if _, err := src.Read("Lib/Noted.sysml"); err == nil {
		t.Fatal("a file whose documented line changed was served unverified")
	}
}

// TestLibraryEntriesAreKeyedByLibraryPath ties the declared library entries to
// the names a libs.Source lists them under.
func TestLibraryEntriesAreKeyedByLibraryPath(t *testing.T) {
	overlay, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	under := overlay.Under(LibraryRoot)
	if len(under) == 0 {
		t.Fatal("no correction is declared for the bundled library")
	}
	for rel, entries := range under {
		if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(LibraryRoot), filepath.FromSlash(rel))); err != nil {
			t.Errorf("%s: %v", rel, err)
		}
		for _, entry := range entries {
			if !entry.Corrects() {
				t.Errorf("%s is documented only yet offered for substitution", entry.ID)
			}
		}
	}
	all, want := overlay.EntriesUnder(LibraryRoot), 0
	for _, entry := range overlay.Entries() {
		if strings.HasPrefix(entry.Path, LibraryRoot+"/") {
			want++
		}
	}
	if Count(all) != want {
		t.Errorf("EntriesUnder(LibraryRoot) holds %d entries, want the %d library entries", Count(all), want)
	}
	if Count(all) <= Count(under) {
		t.Errorf("EntriesUnder holds %d entries, Under %d: the documented-only library entries are not verified", Count(all), Count(under))
	}
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
