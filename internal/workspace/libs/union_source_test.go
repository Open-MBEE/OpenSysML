package libs

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// listCountingSource counts the listings a source is asked for.
type listCountingSource struct {
	Source
	lists int
}

func (s *listCountingSource) List() []string {
	s.lists++
	return s.Source.List()
}

func writeLibraryFiles(t testing.TB, dir string, count int) {
	t.Helper()
	for i := range count {
		path := filepath.Join(dir, fmt.Sprintf("File%04d.sysml", i))
		if err := os.WriteFile(path, []byte("package P;\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestUnionSourceReadsFromOneListing(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	writeLibraryFiles(t, first, 3)
	writeLibraryFiles(t, second, 5)
	a, b := &listCountingSource{Source: NewDirSource(first)}, &listCountingSource{Source: NewDirSource(second)}
	union := &unionSource{sources: []Source{a, b}}

	names := union.List()
	if len(names) != 5 {
		t.Fatalf("listed %d files, want the 5 distinct names", len(names))
	}
	for _, name := range names {
		if _, err := union.Read(name); err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		union.TierOf(name)
	}
	if a.lists != 1 || b.lists != 1 {
		t.Fatalf("sources listed %d and %d times, want once each for one listing and its reads", a.lists, b.lists)
	}
}

func TestUnionSourceSeesFilesThatArriveAndLeave(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	writeLibraryFiles(t, first, 1)
	writeLibraryFiles(t, second, 1)
	if err := os.WriteFile(filepath.Join(second, "File0000.sysml"), []byte("package Second;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	union := &unionSource{sources: []Source{NewDirSource(first), NewDirSource(second)}}
	union.List()

	if err := os.WriteFile(filepath.Join(second, "Late.sysml"), []byte("package Late;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if data, err := union.Read("Late.sysml"); err != nil || string(data) != "package Late;\n" {
		t.Fatalf("a file added after the listing reads as %q, %v", data, err)
	}
	if err := os.Remove(filepath.Join(first, "File0000.sysml")); err != nil {
		t.Fatal(err)
	}
	if data, err := union.Read("File0000.sysml"); err != nil || string(data) != "package Second;\n" {
		t.Fatalf("a file that left the first source reads as %q, %v; want the second source's", data, err)
	}
	if _, err := union.Read("Absent.sysml"); err == nil {
		t.Fatal("a file no source lists was read")
	}
}

func BenchmarkUnionSourceReadAll(b *testing.B) {
	for _, count := range []int{50, 400} {
		first, second := b.TempDir(), b.TempDir()
		writeLibraryFiles(b, first, count)
		writeLibraryFiles(b, second, count)
		b.Run(fmt.Sprintf("files=%d", count), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				union := &unionSource{sources: []Source{NewDirSource(first), NewDirSource(second)}}
				for _, name := range union.List() {
					if _, err := union.Read(name); err != nil {
						b.Fatal(err)
					}
					union.TierOf(name)
				}
			}
		})
	}
}
