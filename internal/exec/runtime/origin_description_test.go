package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/tests/fixtures"
)

// TestOriginStringMatchesNodeDescription pins program.Origin.String() to what
// nodeDescription says of the same node, over every action the conformance fixtures lower.
func TestOriginStringMatchesNodeDescription(t *testing.T) {
	conformanceDir := filepath.Join("testdata", "conformance")
	entries, err := os.ReadDir(conformanceDir)
	if err != nil {
		t.Fatalf("failed to read conformance directory: %v", err)
	}
	checked := 0
	for _, entry := range entries {
		if !isConformanceCase(entry.Name()) {
			continue
		}
		checked += originDescriptionsOfCase(t, conformanceDir, entry.Name())
	}
	if checked == 0 {
		t.Fatal("no action node was checked")
	}
	t.Logf("checked %d action nodes", checked)
}

// originDescriptionsOfCase lowers every action a fixture declares and checks each
// node's origin, reporting how many nodes it checked.
func originDescriptionsOfCase(t *testing.T, conformanceDir, fileName string) int {
	t.Helper()
	caseName := strings.TrimSuffix(fileName, ".expected.json")
	path := fixtures.ModelPath(conformanceDir, caseName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	expectedData, err := os.ReadFile(filepath.Join(conformanceDir, fileName))
	if err != nil {
		t.Fatalf("failed to read expectation of %s: %v", caseName, err)
	}
	var expected ExpectedOutcome
	if err := json.Unmarshal(expectedData, &expected); err != nil {
		t.Fatalf("%s: failed to parse expected.json: %v", caseName, err)
	}
	src := source.New(path, data)
	file := parser.New(src).ParseFile()
	idx, _ := indexCaseDocuments(t, conformanceDir, src, file, expected)
	resolver := resolve.New(idx)
	checked := 0
	var visit func(scope *symbols.Scope)
	visit = func(scope *symbols.Scope) {
		for _, sym := range scope.Members() {
			if declaresAction(sym.Decl) {
				graph, _ := lower.ToActionGraphWith(sym.Decl, DeclScope(sym), resolver)
				checked += checkOriginDescriptions(t, caseName, graph)
			}
		}
		for _, child := range scope.Children() {
			visit(child)
		}
	}
	visit(idx.DocumentRoot(path))
	return checked
}

func declaresAction(decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefAction
	case *ast.Usage:
		return d.Kind == ast.UsageAction
	}
	return false
}

// checkOriginDescriptions checks every node of a graph and of the block graphs
// its bodies state, reporting how many it checked.
func checkOriginDescriptions(t *testing.T, caseName string, graph *lower.ActionGraph) int {
	t.Helper()
	if graph == nil {
		return 0
	}
	checked := 0
	for _, node := range graph.Nodes {
		id, ok := graph.ID[node]
		if !ok {
			t.Errorf("%s: %s has no id", caseName, nodeDescription(node))
			continue
		}
		if got, want := graph.Origins[id].String(), nodeDescription(node); got != want {
			t.Errorf("%s: Origin.String() = %q, nodeDescription = %q", caseName, got, want)
		}
		checked++
	}
	for _, body := range graph.Bodies {
		for _, stmt := range body {
			if block, ok := stmt.(lower.Block); ok {
				checked += checkOriginDescriptions(t, caseName, block.Graph)
			}
		}
	}
	return checked
}
