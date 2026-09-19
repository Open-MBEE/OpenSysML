package errata

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const repoRoot = "../../../.."

// published are the roots the tests declare entries under: the corpus root the
// registry names, and the library's.
var published = []string{"examples/pilot-corpora", LibraryRoot}

func TestLibraryIsAccepted(t *testing.T) {
	overlay, err := Library()
	if err != nil {
		t.Fatalf("load the library's declared errata: %v", err)
	}
	if len(overlay.Entries()) == 0 {
		t.Fatal("the library declares no entry, so nothing it claims can be checked")
	}
	for _, entry := range overlay.Entries() {
		if !strings.HasPrefix(entry.Path, LibraryRoot+"/") {
			t.Errorf("%s lies outside the bundled library: %s", entry.ID, entry.Path)
		}
	}
}

// TestLibraryEntriesMatchThePublishedLibrary is what stops an entry from rotting
// when the library is re-vendored: every entry must still match the bytes on disk.
func TestLibraryEntriesMatchThePublishedLibrary(t *testing.T) {
	for _, entry := range LibraryEntries() {
		content, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(entry.Path))) // #nosec G304 -- the path comes from the declared entries
		if err != nil {
			t.Fatalf("read %s: %v", entry.Path, err)
		}
		if _, err := Apply(entry, content); err != nil {
			t.Fatalf("an entry no longer matches the published library: %v", err)
		}
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
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			entry := mutate(valid)
			if err := entry.Validate(); err == nil {
				t.Fatalf("%s was accepted", name)
			}
			if _, err := New(published, []Entry{entry}); err == nil {
				t.Fatalf("%s was accepted into an overlay", name)
			}
		})
	}
	t.Run("our own material", func(t *testing.T) {
		entry := valid
		entry.Path = "testdata/passes/constraints.sysml"
		if _, err := New(published, []Entry{entry}); err == nil {
			t.Fatal("a correction to our own material was accepted into an overlay")
		}
		if _, err := New(published, []Entry{valid}); err != nil {
			t.Fatalf("the control entry must be accepted under its root: %v", err)
		}
	})
}

func TestDuplicateEntriesAreRejected(t *testing.T) {
	entry := LibraryEntries()[0]
	if _, err := New(published, []Entry{entry, entry}); err == nil {
		t.Fatal("the same entry was accepted twice")
	}
	other := entry
	other.ID = entry.ID + "b"
	if _, err := New(published, []Entry{entry, other}); err == nil {
		t.Fatal("two entries covering one line were accepted")
	}
	other.Line++
	if _, err := New(published, []Entry{entry, other}); err != nil {
		t.Fatalf("two entries covering different lines of one file were refused: %v", err)
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
	overlay, err := New(published, []Entry{entry})
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
	overlay, err := New(published, []Entry{entry, documented})
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
	overlay, err := Library()
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
