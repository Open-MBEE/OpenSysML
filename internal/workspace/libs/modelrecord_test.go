package libs

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func sampleModelRecord(message string) *ModelRecord {
	return &ModelRecord{Documents: []ModelDocument{
		{Name: "a.sysml", Diagnostics: []diag.Diagnostic{{
			Severity: diag.SeverityWarning, Span: source.Span{Offset: 3, Len: 4}, Message: message, Code: "c", Source: "pass",
			Fixes:    []diag.Fix{{Title: "fix", Edits: []diag.Edit{{Span: source.Span{Offset: 3, Len: 4}, NewText: "x"}}}},
			Notation: true,
		}}},
		{Name: "b.sysml"},
	}}
}

func TestModelRecordRoundTrips(t *testing.T) {
	c := &Cache{dir: t.TempDir()}
	key := c.ModelKey("digest", "library")
	if _, ok := c.LoadModel(key); ok {
		t.Fatal("a record nothing stored was loaded")
	}
	want := sampleModelRecord("m")
	if err := c.StoreModel(key, want); err != nil {
		t.Fatalf("StoreModel: %v", err)
	}
	got, ok := c.LoadModel(key)
	if !ok || !reflect.DeepEqual(got, want) {
		t.Fatalf("loaded %+v, stored %+v", got, want)
	}
}

func TestModelKeyNamesTheModelTheLibraryAndTheBuild(t *testing.T) {
	c := &Cache{dir: t.TempDir()}
	key := c.ModelKey("digest", "library")
	if key == c.ModelKey("other", "library") || key == c.ModelKey("digest", "other") {
		t.Fatal("two models, or one model over two libraries, share a key")
	}
	if !strings.Contains(key, "-b"+buildID()) {
		t.Fatalf("key %q does not name the build", key)
	}
	if err := os.WriteFile(c.path(key), []byte("not gob"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.LoadModel(key); ok {
		t.Fatal("an undecodable record was loaded")
	}
}

// Past the bound the least recently used model records go, and only model records.
func TestPruneModelsEvictsTheLeastRecentlyUsed(t *testing.T) {
	c := &Cache{dir: t.TempDir()}
	keys := []string{c.ModelKey("oldest", "l"), c.ModelKey("older", "l"), c.ModelKey("newest", "l")}
	var size int64
	for i, key := range keys {
		if err := c.StoreModel(key, sampleModelRecord("m")); err != nil {
			t.Fatalf("StoreModel: %v", err)
		}
		info, err := os.Stat(c.path(key))
		if err != nil {
			t.Fatal(err)
		}
		size = info.Size()
		used := time.Now().Add(time.Duration(i-len(keys)) * time.Hour)
		if err := os.Chtimes(c.path(key), used, used); err != nil {
			t.Fatal(err)
		}
	}
	library := c.keyFor("a.kerml", []byte("content"), "set")
	if err := c.Store(library, sampleRecord("x")); err != nil {
		t.Fatalf("Store: %v", err)
	}
	old := time.Now().Add(-24 * time.Hour)
	if err := os.Chtimes(c.path(library), old, old); err != nil {
		t.Fatal(err)
	}
	// A hit dates the record, so the oldest written is now the most recently used.
	if _, ok := c.LoadModel(keys[0]); !ok {
		t.Fatal("LoadModel missed")
	}
	c.pruneModels(2 * size)
	for i, kept := range []bool{true, false, true} {
		if _, ok := c.LoadModel(keys[i]); ok != kept {
			t.Errorf("record %d kept=%v, want %v", i, ok, kept)
		}
	}
	if _, ok := c.Load(library); !ok {
		t.Error("a library record was evicted with the model records")
	}
	c.pruneModels(0)
	if _, ok := c.LoadModel(keys[2]); ok {
		t.Error("a record survived a bound of nothing")
	}
}

// A store evicts once what was stored since the last eviction could pass the bound.
func TestStoreModelEvictsPastTheBound(t *testing.T) {
	c := &Cache{dir: t.TempDir()}
	first := c.ModelKey("first", "l")
	if err := c.StoreModel(first, sampleModelRecord("m")); err != nil {
		t.Fatalf("StoreModel: %v", err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(c.path(first), old, old); err != nil {
		t.Fatal(err)
	}
	big := sampleModelRecord(strings.Repeat("m", maxModelRecordBytes))
	last := c.ModelKey("last", "l")
	if err := c.StoreModel(last, big); err != nil {
		t.Fatalf("StoreModel: %v", err)
	}
	if _, ok := c.LoadModel(first); ok {
		t.Error("the least recently used record survived a store past the bound")
	}
	if c.modelBytesStored.Load() != 0 {
		t.Error("the eviction did not restart the count of bytes stored")
	}
}

func TestPruneRemovesIdleModelRecords(t *testing.T) {
	c := &Cache{dir: t.TempDir()}
	key := c.ModelKey("digest", "l")
	if err := c.StoreModel(key, sampleModelRecord("m")); err != nil {
		t.Fatalf("StoreModel: %v", err)
	}
	stale := time.Now().Add(-maxIdleAge - time.Hour)
	if err := os.Chtimes(c.path(key), stale, stale); err != nil {
		t.Fatal(err)
	}
	c.Prune()
	if _, ok := c.LoadModel(key); ok {
		t.Fatal("a model record idle past maxIdleAge survived Prune")
	}
}

// A process's first model record prunes the directory, whatever it stores itself.
func TestFirstStoreModelOfAProcessPrunes(t *testing.T) {
	dir := t.TempDir()
	earlier := &Cache{dir: dir}
	stale, kept := earlier.ModelKey("stale", "l"), earlier.ModelKey("kept", "l")
	for _, key := range []string{stale, kept} {
		if err := earlier.StoreModel(key, sampleModelRecord("m")); err != nil {
			t.Fatalf("StoreModel: %v", err)
		}
	}
	old := time.Now().Add(-maxIdleAge - time.Hour)
	if err := os.Chtimes(earlier.path(stale), old, old); err != nil {
		t.Fatal(err)
	}
	later := &Cache{dir: dir}
	if err := later.StoreModel(later.ModelKey("new", "l"), sampleModelRecord("m")); err != nil {
		t.Fatalf("StoreModel: %v", err)
	}
	if _, ok := later.LoadModel(stale); ok {
		t.Error("a record idle past maxIdleAge survived the next process's first store")
	}
	if _, ok := later.LoadModel(kept); !ok {
		t.Error("a record in use was pruned")
	}
}
