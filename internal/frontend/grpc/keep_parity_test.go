package grpc

import (
	"context"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// cachedOrder is the cache's LRU order, oldest first, as a test sees it.
func cachedOrder(c *Cache) []string {
	names := make([]string, 0, len(c.items))
	for el := c.lruList.Back(); el != nil; el = el.Prev() {
		names = append(names, el.Value.(*cacheEntry).key)
	}
	return names
}

func twoInputSet(lib string) []sourceInput {
	libInput, _ := inlineInput("lib.sysml", "", lib)
	topInput, _ := inlineInput("top.sysml", "", sourcesTop)
	return []sourceInput{libInput, topInput}
}

// An undo to cached documents reuses the kept answer's stamps after the lineage
// advanced: no document is affected, and no entry is promoted or inserted.
func TestParseModelKeepingTransientUndoParity(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	ctx := context.Background()
	mode := diag.ConformanceModeOf(false)

	// The first parse notes the set; the second answers from its workspace, so
	// the base carries the stamps a comparison reads.
	if _, _, err := srv.parseModelKeeping(ctx, twoInputSet(sourcesLibrary), mode, true, true); err != nil {
		t.Fatalf("first parse: %v", err)
	}
	baseInputs := twoInputSet(sourcesLibrary + "// base\n")
	baseHash, base, err := srv.parseModelKeeping(ctx, baseInputs, mode, true, true)
	if err != nil {
		t.Fatalf("base parse: %v", err)
	}
	if base.analysis == nil {
		t.Fatal("the base was not answered from a lineage, so its stamps cannot be compared")
	}
	// Move the lineage to another state without evicting the base.
	if _, _, err := srv.parseModelKeeping(ctx, twoInputSet(sourcesLibrary+"// advanced\n"), mode, true, true); err != nil {
		t.Fatalf("advancing parse: %v", err)
	}
	before := cachedOrder(srv.cache)

	keepFalse, keepTrue := false, true
	hash, transient, err := srv.parseModelKeeping(ctx, baseInputs, mode, true, keepFalse)
	if err != nil {
		t.Fatalf("transient undo: %v", err)
	}
	if hash != baseHash || transient != base {
		t.Fatalf("a transient undo did not reuse the kept answer for %s", baseHash)
	}
	if got := affectedDocuments(base, transient); len(got) != 0 {
		t.Errorf("an undo to the cached state reports affected documents: %v", got)
	}
	if after := cachedOrder(srv.cache); len(after) != len(before) {
		t.Errorf("a transient call changed the cache from %v to %v", before, after)
	} else {
		for i := range before {
			if before[i] != after[i] {
				t.Errorf("a transient call promoted a cache entry: %v became %v", before, after)
				break
			}
		}
	}

	_, kept, err := srv.parseModelKeeping(ctx, baseInputs, mode, true, keepTrue)
	if err != nil {
		t.Fatalf("kept undo: %v", err)
	}
	if kept != transient {
		t.Error("a kept undo did not answer what the transient undo answered")
	}

	// A transient parse of a state not held anywhere still answers, without
	// inserting it into the parsed-model cache.
	unseenHash, unseen, err := srv.parseModelKeeping(ctx, twoInputSet(sourcesLibrary+"// unseen\n"), mode, true, keepFalse)
	if err != nil {
		t.Fatalf("transient new state: %v", err)
	}
	if unseen == nil {
		t.Fatal("a transient new state answered no model")
	}
	if _, ok := srv.cache.Peek(unseenHash); ok {
		t.Error("a transient parse inserted its model into the cache")
	}
}
