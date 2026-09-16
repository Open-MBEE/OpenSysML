package libs

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/errata"
)

// TestBundledSourceIsThePublishedTextPlusTheDeclaredErrata pins what a process
// loads: every published file, byte-identical except at the lines the errata
// registry corrects, and the published source unchanged by the reads.
func TestBundledSourceIsThePublishedTextPlusTheDeclaredErrata(t *testing.T) {
	overlay, err := errata.Load()
	if err != nil {
		t.Fatalf("load the errata registry: %v", err)
	}
	corrections := overlay.Under(errata.LibraryRoot)
	if len(corrections) == 0 {
		t.Fatal("the registry corrects nothing in the bundled library, so this test proves nothing")
	}
	published, bundled := EmbeddedSource(), BundledSource()
	if got, want := bundled.List(), published.List(); !equalStrings(got, want) {
		t.Fatalf("the bundled source lists other files than the published one: %v", got)
	}
	changed := 0
	for _, name := range published.List() {
		before, err := published.Read(name)
		if err != nil {
			t.Fatal(err)
		}
		loaded, err := bundled.Read(name)
		if err != nil {
			t.Fatalf("read %s from the bundled source: %v", name, err)
		}
		after, err := published.Read(name)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Fatalf("%s: the published text changed under a bundled read", name)
		}
		entries := corrections[name]
		if len(entries) == 0 {
			if !bytes.Equal(before, loaded) {
				t.Errorf("%s: no correction is declared, yet the bundled text differs", name)
			}
			continue
		}
		changed++
		want, err := errata.ApplyAll(entries, before)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !bytes.Equal(loaded, want) {
			t.Errorf("%s: the bundled text is not the published text with its corrections applied", name)
		}
		publishedLines, loadedLines := strings.Split(string(before), "\n"), strings.Split(string(loaded), "\n")
		if len(publishedLines) != len(loadedLines) {
			t.Fatalf("%s: a correction changed the line count", name)
		}
		corrected := map[int]errata.Entry{}
		for _, entry := range entries {
			corrected[entry.Line] = entry
		}
		for i := range publishedLines {
			entry, ok := corrected[i+1]
			switch {
			case !ok && publishedLines[i] != loadedLines[i]:
				t.Errorf("%s:%d changed without a declared correction", name, i+1)
			case ok && loadedLines[i] != entry.Corrected:
				t.Errorf("%s:%d reads %q, the entry corrects to %q", name, i+1, loadedLines[i], entry.Corrected)
			}
		}
	}
	if changed != len(corrections) {
		t.Errorf("%d file(s) carry a correction but %d were served corrected", len(corrections), changed)
	}
}

// TestBundledSourceHasItsOwnDigest keeps the snapshot honest: the bundled
// library is not the published one, so the two cannot share a snapshot.
func TestBundledSourceHasItsOwnDigest(t *testing.T) {
	published := NewLoader(EmbeddedSource(), nil).setDigest()
	bundled := NewLoader(BundledSource(), nil).setDigest()
	if published == bundled {
		t.Fatal("the bundled and the published library digest alike")
	}
	if _, err := DecodeSnapshot(stdlibSnapshot, published); !errors.Is(err, ErrSnapshotStale) {
		t.Errorf("the snapshot decoded for the published text: %v", err)
	}
	if _, err := DecodeSnapshot(stdlibSnapshot, bundled); err != nil {
		t.Errorf("the snapshot does not decode for the bundled text: %v", err)
	}
}

// TestDefaultSourceDoesNotCorrectAnOverride pins that a LibraryPathEnvVar
// directory is read as it stands, even when it holds the published text.
func TestDefaultSourceDoesNotCorrectAnOverride(t *testing.T) {
	overlay, err := errata.Load()
	if err != nil {
		t.Fatal(err)
	}
	name := errata.SortedPaths(overlay.Under(errata.LibraryRoot))[0]
	dir := t.TempDir()
	writeLibraryTo(t, EmbeddedSource(), dir)
	t.Setenv(LibraryPathEnvVar, dir)
	want, err := EmbeddedSource().Read(name)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DefaultSource().Read(name)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s from the override differs from the published text", name)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
