package grpc

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/core/envvar"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// IndexPrewarmEnvVar names the variable saying whether the service builds the
// standard library index before the requests that need it. Zero disables
// prewarming, so the first request builds it. The legacy SYSML_GRPC_INDEX_POOL
// name remains accepted; the OPENSYSML_ name wins when both are set.
const IndexPrewarmEnvVar = "OPENSYSML_GRPC_INDEX_POOL"

// DefaultIndexPrewarm is the value IndexPrewarmEnvVar takes when unset: any
// positive value prewarms, since one library index now serves every model.
const DefaultIndexPrewarm = 4

// libraryBuilder builds one frozen index holding only the standard library, with
// the source of the bytes it was built from. A field so tests can count and stub.
type libraryBuilder func() (*symbols.Index, libs.Source)

// buildLibraryIndex decodes the embedded library snapshot when it matches the
// library files; otherwise it loads every file into an index and freezes it.
func buildLibraryIndex() (*symbols.Index, libs.Source) {
	return libs.FrozenLibrary()
}

// libraryBase holds the one standard library index the service's models share.
//
// The library is the same for every model and immutable once loaded, so it is
// built once, frozen, and handed to each model as an overlay carrying that
// model's own document: a model writes only into its overlay, and the base it
// reads through can be read by any number of models at once. Before this, an
// index was handed out whole and mutated by the model that took it, which cost
// one copy of the library per cached model.
type libraryBase struct {
	build libraryBuilder

	mu       sync.Mutex
	base     *symbols.Index
	src      libs.Source // the files base was built from
	stats    baseStats
	building chan struct{} // closed when the prewarm build in flight finishes
}

// baseStats counts what the base did, which is how a test tells a model served
// from a prewarmed library apart from one that loaded the library itself.
type baseStats struct {
	Shared int // models handed an overlay over an already-built base
	Inline int // models that had to build the base on the request path
	Built  int // library indexes built, in the background or inline
}

// newLibraryBase returns a base built on demand by build.
func newLibraryBase(build libraryBuilder) *libraryBase {
	return &libraryBase{build: build}
}

// indexPrewarmFromEnv returns whether the environment asks for prewarming: the
// non-negative integer IndexPrewarmEnvVar holds, or DefaultIndexPrewarm when it
// is unset or empty. An unusable value is an error naming the variable, rather
// than a silently kept default.
func indexPrewarmFromEnv() (int, error) {
	raw := strings.TrimSpace(envvar.Lookup(IndexPrewarmEnvVar))
	if raw == "" {
		return DefaultIndexPrewarm, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("library index pool size must be an integer, got %q (%s)", raw, IndexPrewarmEnvVar)
	}
	if n < 0 {
		return 0, fmt.Errorf("library index pool size must not be negative, got %d (%s)", n, IndexPrewarmEnvVar)
	}
	return n, nil
}

// prewarm builds the library index in the background, so the first model to
// arrive is not the one that pays for it. It returns immediately.
func (b *libraryBase) prewarm() {
	b.mu.Lock()
	if b.base != nil || b.building != nil {
		b.mu.Unlock()
		return
	}
	done := make(chan struct{})
	b.building = done
	b.mu.Unlock()

	go func() {
		defer close(done)
		b.ensure()
	}()
}

// ready reports whether the library index is built.
func (b *libraryBase) ready() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.base != nil
}

// get returns an index carrying the standard library for one model to add its
// document to: an overlay over the shared base, which is built here if nothing
// built it yet, with the source the base was built from.
func (b *libraryBase) get() (*symbols.Index, libs.Source) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.base == nil {
		b.stats.Inline++
		b.buildLocked()
	} else {
		b.stats.Shared++
	}
	return symbols.NewOverlay(b.base), b.src
}

// ensure builds the library index if it is not built yet.
func (b *libraryBase) ensure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.base == nil {
		b.buildLocked()
	}
}

// buildLocked builds the shared index, with the caller holding the lock: a
// second caller waits for the build rather than starting one of its own, since
// two builds of one library would parse what the other was about to cache.
func (b *libraryBase) buildLocked() {
	b.base, b.src = b.build()
	b.stats.Built++
}

// snapshot reports what the base has done so far.
func (b *libraryBase) snapshot() baseStats {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.stats
}

// close waits for a prewarm build in flight and drops the service's reference to
// the shared index; the models still holding it keep resolving, and a later
// request builds a base again.
func (b *libraryBase) close() {
	b.mu.Lock()
	done := b.building
	b.mu.Unlock()
	if done != nil {
		<-done
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	b.base, b.src = nil, nil
	b.building = nil
}
