package libs

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Loader loads library files from a Source and registers them into a target
// index, using a Cache to skip the derivation a load would otherwise repeat.
// Every file is parsed and indexed on every load path, so a library contributes
// its declarations whether or not the cache held anything for it; a record
// carries only the derived facts whose derivation dominates a cold load.
type Loader struct {
	src   Source
	cache *Cache
	// RequireResolved makes LoadAll cache no document with an unresolved
	// specialization target, since a key does not describe the index a record was
	// built in.
	RequireResolved bool
	loaded          []pending // documents loaded this session, awaiting their facts
	digest          string    // memoized digest of the library set (see setDigest)
	hits            int       // documents of the last LoadAll whose facts a record supplied
}

// pending is a parsed document whose facts are not installed yet, with the cache
// record they were restored from or nil when the cache held none.
type pending struct {
	name string
	key  string
	rec  *IndexRecord
}

// libraryFile is one file of the source as read and hashed, with its parse tree
// once parsed; err is the read error when reading it failed.
type libraryFile struct {
	name    string
	content []byte
	sum     [sha256.Size]byte
	root    *ast.RootNamespace
	err     error
}

// NewLoader returns a Loader over src, using cache for persistence.
func NewLoader(src Source, cache *Cache) *Loader {
	return &Loader{src: src, cache: cache}
}

// LoadAll registers every file of the source into idx as library content and
// installs the derived facts of each. It loads the whole set at once because a
// record holds supertype names a sibling file may declare; an incomplete library
// is reported and not cached.
//
// Hashing and parsing are per file and pure, so the files are hashed and parsed
// concurrently; the index is then written in source order, so the result is the
// one a serial load produces.
func (l *Loader) LoadAll(idx *symbols.Index) error {
	var errs []error
	l.hits = 0
	files := l.readAll()
	parallelFor(len(files), func(i int) { files[i].hash(); files[i].parse() })
	if l.digest == "" {
		l.digest = digestOf(files)
	}
	for _, f := range files {
		if f.err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", f.name, f.err))
			continue
		}
		l.add(f, idx)
	}

	// A facade package re-exports what another library package declares.
	idx.ExpandWildcardImports()

	l.installFacts(idx, len(errs) == 0)
	return errors.Join(errs...)
}

// load parses the named library file into idx and marks it library content,
// noting the cache record its facts are restored from when there is one.
func (l *Loader) load(name string, idx *symbols.Index) error {
	f := l.read(name)
	if f.err != nil {
		return f.err
	}
	f.hash()
	f.parse()
	l.add(f, idx)
	return nil
}

// add registers a parsed file into idx and marks it library content, noting the
// cache record its facts are restored from when there is one.
func (l *Loader) add(f libraryFile, idx *symbols.Index) {
	doc := pending{name: f.name}
	if l.cache != nil {
		doc.key = l.cache.keyForSum(f.sum, l.setDigest())
		if rec, ok := l.cache.Load(doc.key); ok {
			doc.rec = rec
		}
	}
	idx.AddDocument(f.name, f.root)
	idx.MarkLibraryDocument(f.name, symbols.LibraryDocument{Tier: TierOf(f.name), Digest: hex.EncodeToString(f.sum[:])})
	l.loaded = append(l.loaded, doc)
}

// bundleTiers maps the directories of the bundled library to their tiers.
var bundleTiers = []struct {
	dir  string
	tier symbols.LibraryTier
}{
	{"Kernel Libraries/Kernel Semantic Library", symbols.TierKernelSemantic},
	{"Kernel Libraries/Kernel Data Type Library", symbols.TierKernelDataType},
	{"Kernel Libraries/Kernel Function Library", symbols.TierKernelFunction},
	{"Systems Library", symbols.TierSystems},
	{"Domain Libraries", symbols.TierDomain},
	{"OpenSysML Libraries", symbols.TierOpenSysML},
}

// TierOf classifies a library file by the directory of the bundle it is under,
// as a Source names it; a file outside the bundle's layout is of no stated tier.
func TierOf(name string) symbols.LibraryTier {
	name = filepath.ToSlash(name)
	for _, b := range bundleTiers {
		if strings.HasPrefix(name, b.dir+"/") {
			return b.tier
		}
	}
	return symbols.TierLibrary
}

// read reads the named file of the source.
func (l *Loader) read(name string) libraryFile {
	f := libraryFile{name: name}
	f.content, f.err = l.src.Read(name)
	return f
}

// readAll reads every file of the source, in List order. Reads are serial: a
// Source need not be safe for concurrent use.
func (l *Loader) readAll() []libraryFile {
	names := l.src.List()
	files := make([]libraryFile, len(names))
	for i, name := range names {
		files[i] = l.read(name)
	}
	return files
}

// hash hashes a file that was read, and is a no-op for one that was not.
func (f *libraryFile) hash() {
	if f.err == nil {
		f.sum = sha256.Sum256(f.content)
	}
}

// parse parses a file that was read, and is a no-op for one that was not.
func (f *libraryFile) parse() {
	if f.err == nil {
		f.root = parser.New(source.New(f.name, f.content)).ParseFile()
	}
}

// parallelFor calls fn for every index below n, from up to GOMAXPROCS
// goroutines, and returns once every call has.
func parallelFor(n int, fn func(i int)) {
	workers := min(runtime.GOMAXPROCS(0), n)
	if workers <= 1 {
		for i := 0; i < n; i++ {
			fn(i)
		}
		return
	}
	var next atomic.Int64
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				i := int(next.Add(1) - 1)
				if i >= n {
					return
				}
				fn(i)
			}
		}()
	}
	wg.Wait()
}

// installFacts installs the derived facts of every loaded document: restored
// from its cache record where there was one, derived where there was not, and
// stored for the next load when store is set.
func (l *Loader) installFacts(idx *symbols.Index, store bool) {
	loaded := l.loaded
	l.loaded = nil

	// Restored facts go in first, so deriving the rest reads the same facts a
	// fully warm load would.
	var missing []pending
	for _, doc := range loaded {
		if doc.rec == nil {
			missing = append(missing, doc)
			continue
		}
		idx.InstallLibraryFacts(doc.name, libraryFacts(doc.rec))
		l.hits++
	}
	if len(missing) == 0 {
		return
	}

	r := resolve.New(idx)
	model := semantics.NewModel(r) // shared: its whole-index memoization is per-model
	type built struct {
		doc      pending
		rec      *IndexRecord
		resolved bool
	}
	records := make([]built, 0, len(missing))
	for _, doc := range missing {
		rec, resolved := recordFromIndex(doc.name, idx, model)
		if rec == nil {
			continue
		}
		records = append(records, built{doc: doc, rec: rec, resolved: resolved})
	}

	// Every record is derived before any is installed, so what one document
	// derives cannot be read back as a fact while another is still deriving.
	for _, b := range records {
		idx.InstallLibraryFacts(b.doc.name, libraryFacts(b.rec))
	}

	if !store || l.cache == nil {
		return
	}
	// Records the library stopped asking for are pruned here, where a write happened.
	defer l.cache.Prune()
	for _, b := range records {
		if l.RequireResolved && !b.resolved {
			continue
		}
		_ = l.cache.Store(b.doc.key, b.rec) // cache write failure is non-fatal
	}
}

// Hits reports how many documents of the last LoadAll had their facts installed
// from a cache record instead of derived, which is how a caller tells a warm load
// from a cold one.
func (l *Loader) Hits() int {
	return l.hits
}

// setDigest hashes the name and content of every file in the source, memoized.
// It goes into the cache key so a record cannot outlive an edit to a sibling file
// it drew a value from, such as a unit reduction following a reference unit
// declared elsewhere. An unreadable file digests as its read error.
func (l *Loader) setDigest() string {
	if l.digest == "" {
		files := l.readAll()
		parallelFor(len(files), func(i int) { files[i].hash() })
		l.digest = digestOf(files)
	}
	return l.digest
}

// digestOf is the set digest of the given files (see setDigest), in name order
// so the result does not depend on the order they were read in.
func digestOf(files []libraryFile) string {
	sorted := append([]libraryFile(nil), files...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].name < sorted[j].name })
	h := sha256.New()
	for _, f := range sorted {
		if f.err != nil {
			fmt.Fprintf(h, "%s\x00!%v\x00", f.name, f.err)
			continue
		}
		fmt.Fprintf(h, "%s\x00%x\x00", f.name, f.sum)
	}
	return hex.EncodeToString(h.Sum(nil))
}
