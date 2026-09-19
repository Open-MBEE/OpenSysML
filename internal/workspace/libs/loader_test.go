package libs

import (
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
