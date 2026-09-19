package libs

import (
	"errors"
	"log/slog"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/envvar"
)

// sharedLibrary is a frozen library index with the source holding exactly the
// bytes it was built from.
type sharedLibrary struct {
	idx *symbols.Index
	src Source
}

// shared holds the frozen library of each library source a process has loaded,
// keyed by the directory LibraryPathEnvVar names ("" for the bundled library),
// so a test pointing at its own library does not get another's.
var shared struct {
	mu   sync.Mutex
	base map[string]sharedLibrary
}

// SharedBase returns the frozen index holding the standard library, built on
// first use and shared by every model afterwards: the library is the same for
// every model and immutable once loaded, so one copy serves them all.
func SharedBase() *symbols.Index {
	idx, _ := SharedLibrary()
	return idx
}

// SharedLibrary returns the shared frozen index (see SharedBase) together with
// the source serving the bytes it was built from. The source answers only for
// the files the index holds, and with the text they had when it was built, so
// a span the index carries always addresses the text the source serves — even
// when a LibraryPathEnvVar override is edited while the process runs.
func SharedLibrary() (*symbols.Index, Source) {
	key := envvar.Lookup(LibraryPathEnvVar)

	shared.mu.Lock()
	defer shared.mu.Unlock()
	if lib, ok := shared.base[key]; ok {
		return lib.idx, lib.src
	}
	idx, src := FrozenLibrary()
	if shared.base == nil {
		shared.base = map[string]sharedLibrary{}
	}
	shared.base[key] = sharedLibrary{idx: idx, src: src}
	return idx, src
}

// FrozenLibrary loads a frozen index holding the standard library, together
// with a source serving exactly the bytes it was built from and nothing else
// (see SharedLibrary). Unlike SharedLibrary it loads on every call.
func FrozenLibrary() (*symbols.Index, Source) {
	return FrozenLibraryFrom(DefaultSource())
}

// FrozenLibraryFrom is FrozenLibrary over the given source, for a consumer
// judging a model against a library other than the one DefaultSource names,
// such as the published text without its declared errata (EmbeddedSource).
func FrozenLibraryFrom(published Source) (*symbols.Index, Source) {
	src := newSnapshotSource(published)
	idx := frozenLibrary(src)
	src.seal()
	return idx, src
}

// frozenLibrary builds a frozen index over src: the embedded snapshot decoded
// when it matches the library files, else the files loaded and frozen. Both
// paths read every file of the source: the snapshot check digests them, and a
// load parses them. A snapshot that fails to decode is logged, since only a
// build defect makes one.
func frozenLibrary(src Source) *symbols.Index {
	idx, err := snapshotIndexOf(src)
	if err == nil {
		return idx
	}
	if !errors.Is(err, ErrSnapshotStale) {
		slog.Warn("stdlib snapshot unreadable, loading the library files", "error", err)
	}
	idx = symbols.NewIndex()
	loadInto(idx, src)
	idx.Freeze()
	return idx
}

// NewModelIndex returns an index for one model to add its documents to: an
// overlay over the shared standard library, which the model reads but cannot
// write to.
func NewModelIndex() *symbols.Index {
	return symbols.NewOverlay(SharedBase())
}
