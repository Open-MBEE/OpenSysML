package libs

import (
	"bytes"
	"debug/elf"
	"debug/macho"
	"errors"
	"io"
	"os"
	"strconv"
)

// linkerBuildID reads the Go linker's build ID from the executable at path as `go tool
// buildid` does (ELF Go note, else the marker at the start of the text); "" when absent.
func linkerBuildID(path string) (string, error) {
	f, err := os.Open(path) // #nosec G304 -- the running executable, named by os.Executable
	if err != nil {
		return "", err
	}
	defer f.Close()
	head := make([]byte, linkerIDReadSize)
	n, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", err
	}
	head = head[:n]
	switch {
	case bytes.HasPrefix(head, []byte(elf.ELFMAG)):
		return elfGoBuildID(f)
	case isMachO(head):
		if id, err := rawGoBuildID(head); id != "" || err != nil {
			return id, err
		}
		return machoGoBuildID(f)
	}
	return rawGoBuildID(head)
}

// linkerIDReadSize is how far into the text the linker places the marker.
const linkerIDReadSize = 32 * 1024

var (
	goBuildIDPrefix = []byte("\xff Go build ID: \"")
	goBuildIDSuffix = []byte("\"\n \xff")
	elfGoNoteName   = []byte("Go\x00\x00")
	machoMagics     = [][]byte{{0xfe, 0xed, 0xfa, 0xce}, {0xfe, 0xed, 0xfa, 0xcf}, {0xce, 0xfa, 0xed, 0xfe}, {0xcf, 0xfa, 0xed, 0xfe}}
)

// elfGoNoteType is the type of the note the linker names "Go" (NT_GO_BUILD_ID).
const elfGoNoteType = 4

func isMachO(head []byte) bool {
	for _, m := range machoMagics {
		if bytes.HasPrefix(head, m) {
			return true
		}
	}
	return false
}

// rawGoBuildID finds the quoted marker the linker writes into the text segment.
func rawGoBuildID(data []byte) (string, error) {
	i := bytes.Index(data, goBuildIDPrefix)
	if i < 0 {
		return "", nil
	}
	start := i + len(goBuildIDPrefix)
	j := bytes.Index(data[start:], goBuildIDSuffix)
	if j < 0 {
		return "", errors.New("malformed Go build ID marker")
	}
	id, err := strconv.Unquote(string(data[start-1 : start+j+1]))
	if err != nil {
		return "", errors.New("malformed Go build ID marker")
	}
	return id, nil
}

// elfGoBuildID walks the PT_NOTE segments for the note named "Go".
func elfGoBuildID(f *os.File) (string, error) {
	ef, err := elf.NewFile(f)
	if err != nil {
		return "", err
	}
	for _, p := range ef.Progs {
		if p.Type != elf.PT_NOTE || p.Filesz < 16 {
			continue
		}
		note, err := io.ReadAll(p.Open())
		if err != nil {
			return "", err
		}
		off := p.Off
		for len(note) >= 16 {
			nameSize := ef.ByteOrder.Uint32(note)
			descSize := ef.ByteOrder.Uint32(note[4:])
			typ := ef.ByteOrder.Uint32(note[8:])
			if nameSize == 4 && typ == elfGoNoteType && 16+uint64(descSize) <= uint64(len(note)) && bytes.Equal(note[12:16], elfGoNoteName) {
				return string(note[16 : 16+descSize]), nil
			}
			size := 12 + uint64((nameSize+3)&^3) + uint64((descSize+3)&^3)
			if p.Align > 1 {
				aligned := (off + size + p.Align - 1) &^ (p.Align - 1)
				size = aligned - off
			}
			if size >= uint64(len(note)) {
				break
			}
			off += size
			note = note[size:]
		}
	}
	return "", nil
}

// machoGoBuildID reads the marker from __text when the header pushed it past the first read.
func machoGoBuildID(f *os.File) (string, error) {
	mf, err := macho.NewFile(f)
	if err != nil {
		return "", err
	}
	sect := mf.Section("__text")
	if sect == nil {
		return "", errors.New("Mach-O file has no __text section")
	}
	n := min(sect.Size, linkerIDReadSize)
	buf := make([]byte, n)
	if _, err := f.ReadAt(buf, int64(sect.Offset)); err != nil {
		return "", err
	}
	return rawGoBuildID(buf)
}
