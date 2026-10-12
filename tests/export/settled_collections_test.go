package export_test

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// The encoder's graph says its collections are settled, so writing api-json
// skips ReconcileCollections and reads each collection from its typed triples
// rather than parsing its JSON annotation. This holds the claim to the check it
// replaces: over every model the repository's examples and export fixtures hold,
// one document at a time and each directory's documents together, a copy of the
// graph that does not vouch for itself reconciles to itself unchanged, and
// writes the same api-json byte for byte.
func TestSettledCollectionsAreWhatReconcilingWouldFind(t *testing.T) {
	models := settledModels(t)
	if len(models) == 0 {
		t.Fatal("no models found")
	}
	checked := 0
	for _, files := range models {
		docs, ok := parsedDocuments(files)
		if !ok {
			continue
		}
		graph, err := export.ModelToRDFWith(docs, export.IDQualifiedName)
		if err != nil {
			continue // a model the export refuses is not converted at all
		}
		name := strings.Join(files, ",")
		if !graph.CollectionsSettled() {
			t.Fatalf("%s: the encoder's graph does not say its collections are settled", name)
		}
		unvouched := rdf.NewGraphOf(graph.Triples(), graph.Prefixes)
		reconciled, err := rdf.ReconcileCollections(unvouched)
		if err != nil {
			t.Fatalf("%s: reconciling the encoder's collections: %v", name, err)
		}
		if reconciled != unvouched {
			t.Errorf("%s: reconciling rewrote the encoder's collections, so they were not settled", name)
			continue
		}
		fast, err := export.WriteAPIJSON(graph)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		checkedOut, err := export.WriteAPIJSON(unvouched)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !bytes.Equal(fast, checkedOut) {
			t.Errorf("%s: api-json differs between the settled graph and the checked one", name)
		}
		checked++
	}
	t.Logf("%d models checked", checked)
	if checked < 50 {
		t.Errorf("only %d models converted; the fixtures moved", checked)
	}
}

// settledModels are the SysML models under the examples and the export
// fixtures: each file alone, and each directory's files together.
func settledModels(t *testing.T) [][]string {
	t.Helper()
	byDir := map[string][]string{}
	for _, root := range []string{"../../examples", "testdata"} {
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(path, ".sysml") {
				byDir[filepath.Dir(path)] = append(byDir[filepath.Dir(path)], path)
			}
			return nil
		})
	}
	dirs := make([]string, 0, len(byDir))
	for dir := range byDir {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	var models [][]string
	for _, dir := range dirs {
		files := byDir[dir]
		sort.Strings(files)
		for _, file := range files {
			models = append(models, []string{file})
		}
		if len(files) > 1 {
			models = append(models, files)
		}
	}
	return models
}

// parsedDocuments parses files as one model, reporting false when one does not parse clean.
func parsedDocuments(files []string) ([]export.ModelDocument, bool) {
	docs := make([]export.ModelDocument, 0, len(files))
	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, false
		}
		file := source.New(path, content)
		p := parser.New(file)
		root := p.ParseFile()
		if len(p.Diagnostics) > 0 {
			return nil, false
		}
		docs = append(docs, export.ModelDocument{File: file, Root: root})
	}
	return docs, true
}
