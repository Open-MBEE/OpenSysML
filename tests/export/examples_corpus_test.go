package export_test

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// A corpusFile is one convert fixture or example, converted once for every
// check of the corpus sweep.
type corpusFile struct {
	path        string
	isExample   bool
	source      []byte
	graph       *rdf.Graph
	conversions *conversionCache
}

// A corpusCheck is an invariant the corpus sweep checks on each converted file
// and summarizes once the sweep is over.
type corpusCheck struct {
	name      string
	check     func(t *testing.T, file corpusFile)
	summarize func(t *testing.T)
}

// TestExamplesCorpusInvariants converts every convert fixture and every example
// once and runs each per-file invariant on that one conversion.
func TestExamplesCorpusInvariants(t *testing.T) {
	t.Parallel()
	paths, exampleRoot := apiJSONQualifiedNamePaths(t)
	conversions := corpusConversions(t)
	checks := []corpusCheck{
		mappingQualifiedNamesAreExact(),
		declaredQualifiedNamesMatchOwningNamespace(),
		mappingShapeAgreesWithForwardMapping(),
		elementGraphWritesTheTurtleGraphsElements(),
	}

	var countsMu sync.Mutex
	fixtures, examples, skippedExamples := 0, 0, 0
	t.Cleanup(func() {
		countsMu.Lock()
		defer countsMu.Unlock()
		t.Logf("examples corpus: %d convert fixtures, %d converting examples, %d skipped examples",
			fixtures, examples, skippedExamples)
		for _, check := range checks {
			check.summarize(t)
		}
	})
	for _, path := range paths {
		path := path
		isExample := strings.HasPrefix(path, exampleRoot+string(filepath.Separator))
		t.Run(filepath.ToSlash(path), func(t *testing.T) {
			t.Parallel()
			defer conversions.release(path)
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			graph, err := conversions.sysMLToRDF(path, source)
			if err != nil {
				if isExample {
					countsMu.Lock()
					skippedExamples++
					countsMu.Unlock()
					t.Logf("skipping non-converting example: %v", err)
					return
				}
				t.Fatalf("convert fixture: %v", err)
			}
			countsMu.Lock()
			if isExample {
				examples++
			} else {
				fixtures++
			}
			countsMu.Unlock()
			file := corpusFile{path: path, isExample: isExample, source: source, graph: graph, conversions: conversions}
			for _, check := range checks {
				check := check
				t.Run(check.name, func(t *testing.T) {
					check.check(t, file)
				})
			}
		})
	}
}
