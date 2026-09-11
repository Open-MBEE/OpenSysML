package libs

import (
	"bytes"
	_ "embed" // for the //go:embed directive
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"runtime/debug"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/core/pack"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

//go:generate go run ./gensnapshot -out stdlib.snapshot

// stdlibSnapshot is the bundled library, parsed, indexed, expanded and frozen
// at generation time, so that a process starts by decoding it instead of
// repeating that work. It is derived from the files under stdlib/ by
// `go generate ./internal/core/libs` and never edited by hand.
//
//go:embed stdlib.snapshot
var stdlibSnapshot []byte

// snapshotMagic opens every snapshot, and snapshotFormatVersion changes
// whenever the stream layout does, so a stale blob is refused, not misread.
const (
	snapshotMagic         = "OpenSysML library snapshot\n"
	snapshotFormatVersion = 14
)

// snapshotCRC checksums the index stream, so a damaged byte that still parses
// is refused rather than decoded into a subtly different library.
var snapshotCRC = crc32.MakeTable(crc32.Castagnoli)

// ErrSnapshotStale reports a snapshot built from different library files, or
// in a different format, than the ones in hand.
var ErrSnapshotStale = errors.New("libs: library snapshot does not match the library files")

// BuildSnapshot loads src into a fresh index the way a process without a
// snapshot would, freezes it, and returns the snapshot bytes for it. The
// result is a function of the files alone: the same files give the same bytes.
func BuildSnapshot(src Source) ([]byte, error) {
	loader := NewLoader(src, nil)
	idx := symbols.NewIndex()
	if err := loader.LoadAll(idx); err != nil {
		return nil, err
	}
	idx.Freeze()

	w := pack.NewWriter()
	if err := idx.WriteSnapshot(w); err != nil {
		return nil, err
	}
	digest, stream := loader.setDigest(), w.Bytes()
	out := []byte(snapshotMagic)
	out = binary.AppendUvarint(out, snapshotFormatVersion)
	out = binary.AppendUvarint(out, uint64(len(digest)))
	out = append(out, digest...)
	out = binary.AppendUvarint(out, uint64(crc32.Checksum(stream, snapshotCRC)))
	return append(out, stream...), nil
}

// DecodeSnapshot rebuilds the frozen index a snapshot holds, provided it was
// built from files with the given set digest (see Loader.setDigest) in the
// current format; otherwise it reports ErrSnapshotStale.
func DecodeSnapshot(data []byte, digest string) (*symbols.Index, error) {
	rest, ok := bytes.CutPrefix(data, []byte(snapshotMagic))
	if !ok {
		return nil, fmt.Errorf("%w: not a library snapshot", pack.ErrCorrupt)
	}
	version, n := binary.Uvarint(rest)
	if n <= 0 {
		return nil, fmt.Errorf("%w: format version", pack.ErrCorrupt)
	}
	rest = rest[n:]
	size, n := binary.Uvarint(rest)
	if n <= 0 {
		return nil, fmt.Errorf("%w: digest", pack.ErrCorrupt)
	}
	rest = rest[n:]
	if size > uint64(len(rest)) {
		return nil, fmt.Errorf("%w: digest", pack.ErrCorrupt)
	}
	have := string(rest[:size])
	rest = rest[size:]
	if version != snapshotFormatVersion || have != digest {
		return nil, ErrSnapshotStale
	}
	sum, n := binary.Uvarint(rest)
	if n <= 0 {
		return nil, fmt.Errorf("%w: checksum", pack.ErrCorrupt)
	}
	rest = rest[n:]
	if uint64(crc32.Checksum(rest, snapshotCRC)) != sum {
		return nil, fmt.Errorf("%w: checksum mismatch", pack.ErrCorrupt)
	}
	r, err := pack.NewReader(rest)
	if err != nil {
		return nil, err
	}
	// The index is tens of MiB that live for the whole process; collecting
	// while it is built would only re-mark it, so GC waits until it is done.
	gcPause.Lock()
	defer gcPause.Unlock()
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	return symbols.ReadSnapshot(r)
}

// gcPause serialises the decodes that pause GC, so that each restores the
// setting it found.
var gcPause sync.Mutex

// SnapshotIndex returns the frozen library index decoded from the embedded
// snapshot, when the snapshot was built from exactly the files DefaultSource
// yields. It fails with ErrSnapshotStale when it was not — an edited bundled
// file, a LibraryPathEnvVar override with other content — and the caller
// loads the files instead.
func SnapshotIndex() (*symbols.Index, error) {
	return snapshotIndexOf(DefaultSource())
}

// snapshotIndexOf is SnapshotIndex over the given source.
func snapshotIndexOf(src Source) (*symbols.Index, error) {
	digest := NewLoader(src, nil).setDigest()
	return DecodeSnapshot(stdlibSnapshot, digest)
}
