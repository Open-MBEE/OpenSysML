package export_test

import (
	"path/filepath"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

type convertedSysMLGraph struct {
	graph *rdf.Graph
	err   error
}

var (
	convertedSysMLGraphsMu sync.Mutex
	convertedSysMLGraphs   = map[string]convertedSysMLGraph{}
)

func cachedSysMLToRDF(path string, source []byte) (*rdf.Graph, error) {
	key := filepath.Clean(path)
	convertedSysMLGraphsMu.Lock()
	defer convertedSysMLGraphsMu.Unlock()
	if cached, ok := convertedSysMLGraphs[key]; ok {
		return cached.graph, cached.err
	}
	graph, err := convert.SysMLToRDF(path, source)
	convertedSysMLGraphs[key] = convertedSysMLGraph{graph: graph, err: err}
	return graph, err
}
