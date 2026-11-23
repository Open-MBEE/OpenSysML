package libs

import (
	"crypto/sha256"
	"errors"
	"slices"
	"sort"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

func TestLoaderLoadsBundledLibraryIntoIndex(t *testing.T) {
	c := &Cache{dir: t.TempDir()}
	ld := NewLoader(DefaultSource(), c)
	idx := symbols.NewIndex()

	if err := ld.load("Kernel Libraries/Kernel Data Type Library/ScalarValues.kerml", idx); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := idx.LookupQualified("ScalarValues"); len(got) != 1 {
		t.Fatalf("ScalarValues lookup = %d, want 1", len(got))
	}
	if got := idx.LookupQualified("ScalarValues::Boolean"); len(got) != 1 {
		t.Fatalf("ScalarValues::Boolean lookup = %d, want 1", len(got))
	}
}

func TestLoaderReadErrorPropagates(t *testing.T) {
	c := &Cache{dir: t.TempDir()}
	ld := NewLoader(DefaultSource(), c)
	idx := symbols.NewIndex()
	if err := ld.load("NoSuchLibrary.kerml", idx); err == nil {
		t.Fatal("Load of missing library returned nil error")
	}
}

// Starting the largest file first changes only the order of the work: every
// file is still visited once, in place.
func TestForEachLargestFirstVisitsEveryFileOnce(t *testing.T) {
	files := []libraryFile{
		{name: "small", content: []byte("a")},
		{name: "unread", err: errors.New("unreadable")},
		{name: "large", content: []byte("abcdef")},
		{name: "middle", content: []byte("abc")},
	}
	var mu sync.Mutex
	var seen []string
	forEachLargestFirst(files, func(f *libraryFile) {
		f.hash()
		mu.Lock()
		seen = append(seen, f.name)
		mu.Unlock()
	})
	sort.Strings(seen)
	if want := []string{"large", "middle", "small", "unread"}; !slices.Equal(seen, want) {
		t.Fatalf("visited %v, want each of %v once", seen, want)
	}
	for _, f := range files {
		if f.err == nil && f.sum != sha256.Sum256(f.content) {
			t.Errorf("%s: the visit did not reach the caller's file", f.name)
		}
	}
}
