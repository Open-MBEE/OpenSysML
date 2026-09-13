package grpc

import (
	"fmt"
	"runtime"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// mustNewCache builds a cache, failing the test if construction errors.
func mustNewCache(t *testing.T, maxSize int) *Cache {
	t.Helper()
	cache, err := NewCache(maxSize)
	if err != nil {
		t.Fatalf("NewCache(%d): %v", maxSize, err)
	}
	return cache
}

// mustNewService builds a service, failing the test if construction errors.
func mustNewService(t *testing.T, cacheSize int) *Service {
	t.Helper()
	srv, err := NewService(cacheSize, "test")
	if err != nil {
		t.Fatalf("NewService(%d): %v", cacheSize, err)
	}
	return srv
}

func mustNewServiceWithout(t *testing.T, unavailable ...string) *Service {
	t.Helper()
	srv, err := NewServiceWithUnavailableCapabilitiesForTesting(10, "test", unavailable)
	if err != nil {
		t.Fatalf("NewServiceWithUnavailableCapabilitiesForTesting: %v", err)
	}
	t.Cleanup(srv.Close)
	return srv
}

func TestCachePutGet(t *testing.T) {
	cache := mustNewCache(t, 2) // Max size 2

	hash1 := "abc123"
	model1 := &CachedModel{
		Documents: []*CachedDocument{{Root: &ast.RootNamespace{}}},
		Index:     symbols.NewIndex(),
	}

	cache.Put(hash1, model1)

	got, ok := cache.Get(hash1)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got != model1 {
		t.Error("got different model")
	}
}

func TestCacheMiss(t *testing.T) {
	cache := mustNewCache(t, 2)

	_, ok := cache.Get("nonexistent")
	if ok {
		t.Error("expected cache miss")
	}
}

func TestCacheLRUEviction(t *testing.T) {
	cache := mustNewCache(t, 2)

	model1 := &CachedModel{Documents: []*CachedDocument{{Root: &ast.RootNamespace{}}}}
	model2 := &CachedModel{Documents: []*CachedDocument{{Root: &ast.RootNamespace{}}}}
	model3 := &CachedModel{Documents: []*CachedDocument{{Root: &ast.RootNamespace{}}}}

	cache.Put("hash1", model1)
	cache.Put("hash2", model2)
	cache.Put("hash3", model3) // Should evict hash1

	_, ok := cache.Get("hash1")
	if ok {
		t.Error("expected hash1 to be evicted")
	}

	_, ok = cache.Get("hash2")
	if !ok {
		t.Error("expected hash2 to still be cached")
	}

	_, ok = cache.Get("hash3")
	if !ok {
		t.Error("expected hash3 to be cached")
	}
}

func TestCacheThreadSafety(t *testing.T) {
	cache := mustNewCache(t, 100)
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			model := &CachedModel{Documents: []*CachedDocument{{Root: &ast.RootNamespace{}}}}
			cache.Put(fmt.Sprintf("key%d", n), model)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			cache.Get(fmt.Sprintf("key%d", n))
		}(i)
	}

	wg.Wait()
	// If we reach here without race detector firing, thread safety works
}

func TestCacheInvalidMaxSize(t *testing.T) {
	if _, err := NewCache(0); err == nil {
		t.Error("expected error for maxSize <= 0")
	}
	if _, err := NewService(0, "test"); err == nil {
		t.Error("expected error for cacheSize <= 0")
	}
}

// A request holds a model's worker alone, hands it on warm when it is done, and a request
// arriving while every worker is held gets one of its own rather than waiting.
func TestCachedModelHandsWorkersOn(t *testing.T) {
	model := &CachedModel{Documents: []*CachedDocument{{Root: &ast.RootNamespace{}}}, Index: symbols.NewIndex()}
	model.Index.Freeze()

	first, releaseFirst := model.worker()
	second, releaseSecond := model.worker()
	if first == second {
		t.Fatal("two requests holding workers at once were handed the same one")
	}
	releaseFirst()
	third, releaseThird := model.worker()
	if third != first {
		t.Fatal("a request after the first released was not handed its worker warm")
	}
	releaseThird()
	releaseSecond()
	if want := min(2, maxIdleWorkers()); len(model.idle) != want {
		t.Fatalf("%d idle workers after every release, want %d", len(model.idle), want)
	}

	// What a request failed to resolve is the request's, not the next holder's.
	w, release := model.worker()
	w.Model.Resolver().Diagnostics = append(w.Model.Resolver().Diagnostics, resolve.Diagnostic{Message: "nosuch"})
	release()
	again, releaseAgain := model.worker()
	defer releaseAgain()
	if again != w {
		t.Fatal("the released worker was not the one handed on")
	}
	if got := len(again.Model.Resolver().Diagnostics); got != 0 {
		t.Fatalf("a worker handed on carries %d diagnostics of an earlier request, want none", got)
	}
}

// A burst of requests wider than the machine leaves the model holding no more warm workers
// than can run at once; the rest are let go with the requests that built them.
func TestCachedModelKeepsABoundedNumberOfIdleWorkers(t *testing.T) {
	model := &CachedModel{Documents: []*CachedDocument{{Root: &ast.RootNamespace{}}}, Index: symbols.NewIndex()}
	model.Index.Freeze()

	bound := maxIdleWorkers()
	releases := make([]func(), 0, bound+3)
	for range bound + 3 {
		_, release := model.worker()
		releases = append(releases, release)
	}
	for _, release := range releases {
		release()
	}
	if got := len(model.idle); got != bound {
		t.Fatalf("%d idle workers after a burst of %d, want the bound %d", got, bound+3, bound)
	}
	_, release := model.worker()
	if got := len(model.idle); got != bound-1 {
		t.Fatalf("%d idle workers while a request after the burst holds one, want %d", got, bound-1)
	}

	// The bound follows the parallelism: a pool filled under a wider one shrinks at the next release.
	t.Cleanup(func() { runtime.GOMAXPROCS(bound) })
	runtime.GOMAXPROCS(1)
	release()
	if got := len(model.idle); got != 1 {
		t.Fatalf("%d idle workers after the parallelism fell to 1, want 1", got)
	}
}
