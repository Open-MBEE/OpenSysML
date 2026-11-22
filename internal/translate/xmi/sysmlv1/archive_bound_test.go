package sysmlv1

import (
	"archive/zip"
	"strings"
	"testing"
)

func entryOf(name string, size uint64) *zip.File {
	return &zip.File{FileHeader: zip.FileHeader{Name: name, UncompressedSize64: size}}
}

func TestCheckModelSizeBoundsTheEntriesTogether(t *testing.T) {
	each := uint64(maxEntrySize)
	under := []*zip.File{entryOf("a", each), entryOf("b", each), entryOf("c", each), entryOf("d", each)}
	if err := checkModelSize(under[:2], under[2:]); err != nil {
		t.Fatalf("entries of exactly the bound together: %v", err)
	}
	err := checkModelSize(under, []*zip.File{entryOf("e", 1)})
	if err == nil || !strings.Contains(err.Error(), "together, at e") {
		t.Fatalf("err = %v, want the entry that passed the bound named", err)
	}
}
