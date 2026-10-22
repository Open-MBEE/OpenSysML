package export_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

type sysMLGraphCacheKey struct {
	path   string
	source [sha256.Size]byte
}

type convertedSysMLGraph struct {
	once  sync.Once
	graph *rdf.Graph
	err   error
}

type apiJSONCacheKey struct {
	path   string
	source [sha256.Size]byte
	form   string
	route  string
}

type writtenAPIJSON struct {
	once     sync.Once
	document []byte
	err      error
}

// A conversionCache shares one file's RDF graph, and the API JSON documents
// written from it, between the readers of the same file. It keeps every entry
// until the file is released, so each cache lives only as long as its readers.
type conversionCache struct {
	mu        sync.Mutex
	graphs    map[sysMLGraphCacheKey]*convertedSysMLGraph
	documents map[apiJSONCacheKey]*writtenAPIJSON
}

func newConversionCache() *conversionCache {
	return &conversionCache{
		graphs:    map[sysMLGraphCacheKey]*convertedSysMLGraph{},
		documents: map[apiJSONCacheKey]*writtenAPIJSON{},
	}
}

// fixtureConversions is the process-wide cache of the small convert fixtures
// under testdata, which many tests read in turn.
var fixtureConversions = newConversionCache()

// corpusConversions is the cache of one sweep over the examples corpus. A sweep
// reads each file in one subtest, which releases the file when it is done, so
// a sweep holds the graphs and documents of its in-flight files only: under the
// race detector, keeping every example's conversion for the whole run exhausted
// the memory of the CI runner.
func corpusConversions(t *testing.T) *conversionCache {
	t.Helper()
	cache := newConversionCache()
	t.Cleanup(cache.releaseAll)
	return cache
}

// cachedSysMLToRDF converts a fixture through the process-wide cache.
func cachedSysMLToRDF(path string, source []byte) (*rdf.Graph, error) {
	return fixtureConversions.sysMLToRDF(path, source)
}

// cachedAPIJSON returns a fixture's immutable cached output; callers must not
// modify it.
func cachedAPIJSON(path string, source []byte, form, route string) ([]byte, error) {
	return fixtureConversions.apiJSON(path, source, form, route)
}

func (c *conversionCache) sysMLToRDF(path string, source []byte) (*rdf.Graph, error) {
	key := sysMLGraphCacheKey{path: filepath.Clean(path), source: sha256.Sum256(source)}
	c.mu.Lock()
	entry, ok := c.graphs[key]
	if !ok {
		entry = &convertedSysMLGraph{}
		c.graphs[key] = entry
	}
	c.mu.Unlock()
	entry.once.Do(func() {
		entry.graph, entry.err = convert.SysMLToRDF(path, source)
	})
	return entry.graph, entry.err
}

// apiJSON returns immutable cached output; callers must not modify it.
func (c *conversionCache) apiJSON(path string, source []byte, form, route string) ([]byte, error) {
	key := apiJSONCacheKey{
		path: filepath.Clean(path), source: sha256.Sum256(source), form: form, route: route,
	}
	c.mu.Lock()
	entry, ok := c.documents[key]
	if !ok {
		entry = &writtenAPIJSON{}
		c.documents[key] = entry
	}
	c.mu.Unlock()
	entry.once.Do(func() {
		switch route {
		case "graph":
			graph, err := c.sysMLToRDF(path, source)
			if err == nil {
				entry.document, entry.err = writeAPIJSONForm(graph, form)
			} else {
				entry.err = err
			}
		case "semantic":
			entry.document, entry.err = semanticAPIJSON(path, source, form)
		default:
			entry.err = fmt.Errorf("unknown API JSON route %q", route)
		}
	})
	return entry.document, entry.err
}

// release forgets every graph and document of one file.
func (c *conversionCache) release(path string) {
	path = filepath.Clean(path)
	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range c.graphs {
		if key.path == path {
			delete(c.graphs, key)
		}
	}
	for key := range c.documents {
		if key.path == path {
			delete(c.documents, key)
		}
	}
}

func (c *conversionCache) releaseAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.graphs = map[sysMLGraphCacheKey]*convertedSysMLGraph{}
	c.documents = map[apiJSONCacheKey]*writtenAPIJSON{}
}

// retained counts the files whose conversions the cache still holds.
func (c *conversionCache) retained() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	files := map[string]bool{}
	for key := range c.graphs {
		files[key.path] = true
	}
	for key := range c.documents {
		files[key.path] = true
	}
	return len(files)
}

// semanticAPIJSON writes a file's API JSON through the semantic route, in the
// standard form or compacted from it.
func semanticAPIJSON(path string, source []byte, form string) ([]byte, error) {
	format, err := convert.FormatOfPath(path)
	if err != nil {
		return nil, err
	}
	standard, err := convert.Convert(path, source, format, convert.FormatAPIJSON)
	if err != nil {
		return nil, err
	}
	if form == "standard" {
		return standard, nil
	}
	graph, err := export.ReadAPIJSON(standard)
	if err != nil {
		return nil, err
	}
	return writeAPIJSONForm(graph, form)
}

func writeAPIJSONForm(graph *rdf.Graph, form string) ([]byte, error) {
	switch form {
	case "standard":
		return export.WriteAPIJSON(graph)
	case "compact":
		return export.WriteAPIJSONCompact(graph, export.CompactAPIJSONOptions{})
	default:
		return nil, fmt.Errorf("unknown API JSON form %q", form)
	}
}

func TestCachedAPIJSONSeparatesSourceContentsAtSamePath(t *testing.T) {
	path := filepath.Join("testdata", "cache-content-key.sysml")
	firstSource := []byte("package P { part def A; }")
	secondSource := []byte("package P { part def B; }")

	firstGraph, err := cachedSysMLToRDF(path, firstSource)
	if err != nil {
		t.Fatalf("convert first source: %v", err)
	}
	secondGraph, err := cachedSysMLToRDF(path, secondSource)
	if err != nil {
		t.Fatalf("convert second source: %v", err)
	}
	if firstGraph == secondGraph {
		t.Fatal("same-path source changes reused the cached RDF graph")
	}

	firstDocument, err := cachedAPIJSON(path, firstSource, "standard", "graph")
	if err != nil {
		t.Fatalf("write first source: %v", err)
	}
	secondDocument, err := cachedAPIJSON(path, secondSource, "standard", "graph")
	if err != nil {
		t.Fatalf("write second source: %v", err)
	}
	if bytes.Equal(firstDocument, secondDocument) {
		t.Fatal("same-path source changes reused the cached API JSON")
	}
}

func TestConversionCacheReleasesAFileWithItsGraphAndDocuments(t *testing.T) {
	cache := newConversionCache()
	first := filepath.Join("testdata", "cache-release-first.sysml")
	second := filepath.Join("testdata", "cache-release-second.sysml")
	source := []byte("package P { part def A; }")

	firstGraph, err := cache.sysMLToRDF(first, source)
	if err != nil {
		t.Fatalf("convert first file: %v", err)
	}
	if _, err := cache.apiJSON(first, source, "standard", "graph"); err != nil {
		t.Fatalf("write first file: %v", err)
	}
	if _, err := cache.sysMLToRDF(second, source); err != nil {
		t.Fatalf("convert second file: %v", err)
	}
	if got := cache.retained(); got != 2 {
		t.Fatalf("cache retains %d files before the release; want 2", got)
	}

	cache.release(first)
	if got := cache.retained(); got != 1 {
		t.Fatalf("cache retains %d files after releasing one; want 1", got)
	}
	again, err := cache.sysMLToRDF(first, source)
	if err != nil {
		t.Fatalf("convert first file again: %v", err)
	}
	if again == firstGraph {
		t.Fatal("a released file's graph was served from the cache")
	}

	cache.releaseAll()
	if got := cache.retained(); got != 0 {
		t.Fatalf("cache retains %d files after releasing all; want 0", got)
	}
}
