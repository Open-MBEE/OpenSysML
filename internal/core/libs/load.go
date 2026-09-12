package libs

import (
	"log/slog"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// LoadInto loads every standard library file into idx, using the symbol cache
// when it is available. A failure is non-fatal: it is logged, and the index
// stays usable for a model that does not depend on the file that failed.
func LoadInto(idx *symbols.Index) {
	loadInto(idx, DefaultSource())
}

// loadInto is LoadInto over the given source.
func loadInto(idx *symbols.Index, src Source) {
	cache, err := NewCache()
	if err != nil {
		slog.Warn("stdlib symbol cache unavailable, loading without cache", "error", err)
		cache = nil
	}
	if err := NewLoader(src, cache).LoadAll(idx); err != nil {
		slog.Warn("failed to load stdlib files", "error", err)
	}
}

// Version identifies the standard library the host loads: `sha256:` and the set
// digest of every file of DefaultSource, so two hosts embedding the same files
// report the same version and an edited or overridden library reports another.
func Version() string {
	return libraryVersion()
}

var libraryVersion = sync.OnceValue(func() string {
	return "sha256:" + NewLoader(DefaultSource(), nil).setDigest()
})
