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

var (
	convertedSysMLGraphsMu sync.Mutex
	convertedSysMLGraphs   = map[sysMLGraphCacheKey]*convertedSysMLGraph{}
	writtenAPIJSONMu       sync.Mutex
	writtenAPIJSONByKey    = map[apiJSONCacheKey]*writtenAPIJSON{}
)

func cachedSysMLToRDF(path string, source []byte) (*rdf.Graph, error) {
	key := sysMLGraphCacheKey{path: filepath.Clean(path), source: sha256.Sum256(source)}
	convertedSysMLGraphsMu.Lock()
	entry, ok := convertedSysMLGraphs[key]
	if !ok {
		entry = &convertedSysMLGraph{}
		convertedSysMLGraphs[key] = entry
	}
	convertedSysMLGraphsMu.Unlock()
	entry.once.Do(func() {
		entry.graph, entry.err = convert.SysMLToRDF(path, source)
	})
	return entry.graph, entry.err
}

// cachedAPIJSON returns immutable cached output; callers must not modify it.
func cachedAPIJSON(path string, source []byte, form, route string) ([]byte, error) {
	key := apiJSONCacheKey{
		path: filepath.Clean(path), source: sha256.Sum256(source), form: form, route: route,
	}
	entry := cachedAPIJSONEntry(key)
	entry.once.Do(func() {
		switch route {
		case "graph":
			graph, err := cachedSysMLToRDF(path, source)
			if err == nil {
				entry.document, entry.err = writeCachedAPIJSON(graph, form)
			} else {
				entry.err = err
			}
		case "semantic":
			format, err := convert.FormatOfPath(path)
			if err != nil {
				entry.err = err
				break
			}
			standard, err := convert.Convert(path, source, format, convert.FormatAPIJSON)
			if err != nil {
				entry.err = err
				break
			}
			if form == "standard" {
				entry.document = standard
				break
			}
			graph, err := export.ReadAPIJSON(standard)
			if err == nil {
				entry.document, entry.err = writeCachedAPIJSON(graph, form)
			} else {
				entry.err = err
			}
		default:
			entry.err = fmt.Errorf("unknown API JSON route %q", route)
		}
	})
	return entry.document, entry.err
}

func cachedAPIJSONForGraph(path, form, route string, graph *rdf.Graph) ([]byte, error) {
	key := apiJSONCacheKey{path: filepath.Clean(path), form: form, route: route}
	entry := cachedAPIJSONEntry(key)
	entry.once.Do(func() {
		entry.document, entry.err = writeCachedAPIJSON(graph, form)
	})
	return entry.document, entry.err
}

func cachedAPIJSONEntry(key apiJSONCacheKey) *writtenAPIJSON {
	writtenAPIJSONMu.Lock()
	defer writtenAPIJSONMu.Unlock()
	entry, ok := writtenAPIJSONByKey[key]
	if !ok {
		entry = &writtenAPIJSON{}
		writtenAPIJSONByKey[key] = entry
	}
	return entry
}

func writeCachedAPIJSON(graph *rdf.Graph, form string) ([]byte, error) {
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
