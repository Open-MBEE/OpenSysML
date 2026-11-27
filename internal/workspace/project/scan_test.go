package project

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

const partsProject = `{"name": "Parts", "publisher": "acme", "version": "1.0.0"}`

// TestScanWalksALibraryRootOnce shows a shared Scan answering from its first
// walk of a root, and a discovery without one seeing the disk as it is.
func TestScanWalksALibraryRootOnce(t *testing.T) {
	store := t.TempDir()
	writeProject(t, filepath.Join(store, "parts"), partsProject)
	scanned := Discovery{Roots: []string{store}, Scan: NewScan()}
	if got := scanned.Around(nil).Used; len(got) != 1 {
		t.Fatalf("found %v, want the one installed project", projectDirs(got))
	}

	writeProject(t, filepath.Join(store, "late"), `{"name": "Late", "publisher": "acme", "version": "1.0.0"}`)
	if got := scanned.Around(nil).Used; len(got) != 1 {
		t.Errorf("the same scan found %v, want its first walk's one project", projectDirs(got))
	}
	for name, d := range map[string]Discovery{
		"no scan":    {Roots: []string{store}},
		"a new scan": {Roots: []string{store}, Scan: NewScan()},
	} {
		if got := d.Around(nil).Used; len(got) != 2 {
			t.Errorf("%s found %v, want both installed projects", name, projectDirs(got))
		}
	}
}

// BenchmarkDiscoveryAround runs the three discoveries one load makes over a
// library root of many directories.
func BenchmarkDiscoveryAround(b *testing.B) {
	store := b.TempDir()
	for i := range 500 {
		if err := os.Mkdir(filepath.Join(store, fmt.Sprintf("d%04d", i)), 0o750); err != nil {
			b.Fatal(err)
		}
	}
	writeProject(b, filepath.Join(store, "parts"), partsProject)
	for name, scan := range map[string]func() *Scan{"walk each time": func() *Scan { return nil }, "one scan": NewScan} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				d := Discovery{Roots: []string{store}, Scan: scan()}
				for range 3 {
					if got := d.Around(nil).Used; len(got) != 1 {
						b.Fatalf("found %d projects", len(got))
					}
				}
			}
		})
	}
}
