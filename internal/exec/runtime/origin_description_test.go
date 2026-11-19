package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/tests/fixtures"
)

// TestOriginDescriptions pins what a message calls an action node over every action the
// conformance fixtures lower: `node <name>` when named, else `the <NodeKind.Word>`.
func TestOriginDescriptions(t *testing.T) {
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
			t.Errorf("%s: %T %s has no id", caseName, node, ActionNodeName(node))
			continue
		}
		origin, kind := graph.Origins[id], graph.Kinds[id]
		if origin.Kind != kind.Word() {
			t.Errorf("%s: Origin.Kind = %q, want %q", caseName, origin.Kind, kind.Word())
		}
		if origin.Span != node.Span() {
			t.Errorf("%s: Origin.Span = %v, want %v", caseName, origin.Span, node.Span())
		}
		if origin.Doc == "" {
			t.Errorf("%s: %s has no document", caseName, origin)
		}
		want := "the " + kind.Word()
		if name := ActionNodeName(node); name != "" {
			want = "node " + name
		}
		if got := origin.String(); got != want {
			t.Errorf("%s: Origin.String() = %q, want %q", caseName, got, want)
		}
		if got := describeNode(graph, node); got != want {
			t.Errorf("%s: describeNode = %q, want %q", caseName, got, want)
		}
		if got := refOf(graph, node).Origin(); got != origin {
			t.Errorf("%s: NodeRef.Origin() = %v, want %v", caseName, got, origin)
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

// TestDescribeNodeOutsideGraph pins what a message calls a node no graph indexes.
func TestDescribeNodeOutsideGraph(t *testing.T) {
	if got, want := describeNode(nil, nil), (program.Origin{}).String(); got != want {
		t.Errorf("describeNode(nil, nil) = %q, want %q", got, want)
	}
	if got, want := (NodeRef{}).Origin(), (program.Origin{}); got != want {
		t.Errorf("zero NodeRef.Origin() = %v, want the zero Origin", got)
	}
}
