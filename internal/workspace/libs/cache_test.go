package libs

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func sampleRecord(name string) *IndexRecord {
	return &IndexRecord{
		Name: name,
		Facts: []factRecord{
			{FQN: "P::N", Supers: []string{"P::M"}},
			{FQN: "P::M", Unit: &unitFacts{ScaleNum: 1, ScaleDen: 1}},
		},
	}
}

func TestCacheStoreLoadRoundTrip(t *testing.T) {
	c := &Cache{dir: t.TempDir()}
	rec := sampleRecord("a.kerml")
	key := c.keyFor([]byte("content-a"), "set")
	if err := c.Store(key, rec); err != nil {
		t.Fatalf("store: %v", err)
	}
	got, ok := c.Load(key)
	if !ok {
		t.Fatal("Load miss after Store")
	}
	// factRecord holds slices and pointers, so compare the round-tripped fields.
	if got.Name != rec.Name || len(got.Facts) != len(rec.Facts) {
		t.Fatalf("round-trip mismatch: got %+v want %+v", got, rec)
	}
	a, b := got.Facts[0], rec.Facts[0]
	if a.FQN != b.FQN || !slices.Equal(a.Supers, b.Supers) {
		t.Fatalf("fact round-trip mismatch: got %+v want %+v", a, b)
	}
	if unit := got.Facts[1].Unit; unit == nil || unit.ScaleNum != rec.Facts[1].Unit.ScaleNum ||
		unit.ScaleDen != rec.Facts[1].Unit.ScaleDen {
		t.Fatalf("unit facts round-tripped as %+v, want %+v", unit, rec.Facts[1].Unit)
	}
}

func TestCacheLoadUnknownKeyMisses(t *testing.T) {
	c := &Cache{dir: t.TempDir()}
	if _, ok := c.Load(c.keyFor([]byte("never-stored"), "set")); ok {
		t.Fatal("Load returned hit for unknown key")
	}
}

func TestCacheKeyDependsOnContentSetAndVersion(t *testing.T) {
	c := &Cache{dir: t.TempDir()}
	k1 := c.keyFor([]byte("alpha"), "set")
	k2 := c.keyFor([]byte("beta"), "set")
	if k1 == k2 {
		t.Fatal("distinct content produced identical cache keys")
	}
	// A record persists facts derived from sibling files, so the same content in
	// a different library set is a different record.
	if k1 == c.keyFor([]byte("alpha"), "other-set") {
		t.Fatal("distinct library sets produced identical cache keys")
	}
	// A record stored under content "alpha" must not be found by content
	// "beta" (stale-content miss) — the core cache-key invariant.
	if err := c.Store(k1, sampleRecord("x")); err != nil {
		t.Fatalf("store: %v", err)
	}
	if _, ok := c.Load(k2); ok {
		t.Fatal("stale content produced a cache hit")
	}
}

// A record holds facts the code computes rather than reads — a supertype set, a
// unit reduction — so a record written by another build must miss, not be served
// with the answer that build gave.
func TestCacheKeyDependsOnBuildID(t *testing.T) {
	c := &Cache{dir: t.TempDir()}
	key := c.keyFor([]byte("alpha"), "set")
	if err := c.Store(key, sampleRecord("x")); err != nil {
		t.Fatalf("store: %v", err)
	}
	restore := buildID
	buildID = func() string { return "other-build" }
	defer func() { buildID = restore }()
	other := c.keyFor([]byte("alpha"), "set")
	if other == key {
		t.Fatal("distinct builds produced identical cache keys")
	}
	if _, ok := c.Load(other); ok {
		t.Fatal("a record written by another build produced a cache hit")
	}
}

func TestCachePruneRemovesOnlyIdleRecords(t *testing.T) {
	c := &Cache{dir: t.TempDir()}
	idle, live := c.keyFor([]byte("idle"), "set"), c.keyFor([]byte("live"), "set")
	for _, key := range []string{idle, live} {
		if err := c.Store(key, sampleRecord("x")); err != nil {
			t.Fatalf("store: %v", err)
		}
	}
	stale := time.Now().Add(-maxIdleAge - time.Hour)
	if err := os.Chtimes(c.path(idle), stale, stale); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	c.Prune()
	if _, err := os.Stat(c.path(idle)); !os.IsNotExist(err) {
		t.Fatalf("a record idle past maxIdleAge survived Prune: %v", err)
	}
	if _, ok := c.Load(live); !ok {
		t.Fatal("Prune removed a record still in use")
	}
}

// A record is only written once, so its age has to track its last use: a hit
// dates it, or pruning would evict the records of an unchanged library.
func TestCacheLoadDatesTheRecord(t *testing.T) {
	c := &Cache{dir: t.TempDir()}
	key := c.keyFor([]byte("content"), "set")
	if err := c.Store(key, sampleRecord("x")); err != nil {
		t.Fatalf("store: %v", err)
	}
	stale := time.Now().Add(-maxIdleAge - time.Hour)
	if err := os.Chtimes(c.path(key), stale, stale); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	if _, ok := c.Load(key); !ok {
		t.Fatal("Load miss after Store")
	}
	c.Prune()
	if _, ok := c.Load(key); !ok {
		t.Fatal("Prune removed a record the previous load hit")
	}
}

func TestNewCacheCreatesDir(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	c, err := NewCache()
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	if c.dir == "" {
		t.Fatal("NewCache produced empty dir")
	}
	// Store/Load must work against the freshly created dir.
	key := c.keyFor([]byte("z"), "set")
	if err := c.Store(key, sampleRecord("z")); err != nil {
		t.Fatalf("store into new cache dir: %v", err)
	}
	if _, ok := c.Load(key); !ok {
		t.Fatal("Load miss from freshly created cache dir")
	}
}

func TestCacheStoreIsAtomic(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	c, err := NewCache()
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	key := c.keyFor([]byte("some content"), "set")
	if err := c.Store(key, sampleRecord("P")); err != nil {
		t.Fatalf("Store: %v", err)
	}
	// The final file exists and round-trips...
	if _, ok := c.Load(key); !ok {
		t.Fatalf("Load after Store: miss")
	}
	// ...and no temp file is left behind.
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("leftover temp file after Store: %s", e.Name())
		}
	}
}

// TestCacheStoreConcurrentWritersOfOneKey covers what several library builds do
// on a cold cache: they miss on the same keys and store them at once, and each
// store has to publish a whole record rather than truncate a peer's temp file.
func TestCacheStoreConcurrentWritersOfOneKey(t *testing.T) {
	c := &Cache{dir: t.TempDir()}
	rec := sampleRecord("crowded.kerml")
	key := c.keyFor([]byte("content stored by everyone"), "set")

	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = c.Store(key, rec)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("writer %d: %v", i, err)
		}
	}

	got, ok := c.Load(key)
	if !ok {
		t.Fatal("Load miss after concurrent stores: a record was published truncated")
	}
	if got.Name != rec.Name || len(got.Facts) != len(rec.Facts) {
		t.Fatalf("record after concurrent stores: got %+v want %+v", got, rec)
	}
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("cache holds %d files after storing one key, want 1", len(entries))
	}
}

// TestCachePruneRemovesStaleTempFiles covers the temp a crashed store leaves: it
// is not a record, so nothing ever loads it, and only Prune can clear it.
func TestCachePruneRemovesStaleTempFiles(t *testing.T) {
	c := &Cache{dir: t.TempDir()}
	stale := filepath.Join(c.dir, "deadbeef.idx.tmp-123456")
	if err := os.WriteFile(stale, []byte("half a record"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * maxIdleAge)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}
	// A build before the temp file got a name of its own left this spelling.
	legacy := filepath.Join(c.dir, "d00dfeed.idx.tmp")
	if err := os.WriteFile(legacy, []byte("half a record"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(legacy, old, old); err != nil {
		t.Fatal(err)
	}
	fresh := filepath.Join(c.dir, "cafebabe.idx.tmp-654321")
	if err := os.WriteFile(fresh, []byte("a store in flight"), 0o600); err != nil {
		t.Fatal(err)
	}

	c.Prune()

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("Prune kept a temp file no store is writing")
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Error("Prune kept a temp file left by an older build")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("Prune removed a temp file a store may still be writing: %v", err)
	}
}
