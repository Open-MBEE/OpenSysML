package view_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// TestRenderingsDrawEachNodeOnce renders every view under testdata, and every
// fixture's top-level elements in every supported kind, and holds each rendering
// to one node per id with each node placed once.
func TestRenderingsDrawEachNodeOnce(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "*.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	exists := make(map[string]bool, len(paths))
	for _, path := range paths {
		exists[filepath.Base(path)] = true
	}
	kinds := map[view.Kind]bool{}
	for _, files := range fixtureSets(t, paths, exists) {
		renderer, idx := fixtureRenderer(t, files)
		for _, file := range files {
			var tops []*symbols.Symbol
			for _, sym := range idx.DocumentRoot(file).AllMembers() {
				tops = append(tops, sym)
			}
			for _, v := range fixtureViews(idx.DocumentRoot(file)) {
				rendering, err := renderer.Render(v)
				if err != nil {
					continue
				}
				kinds[rendering.Kind] = true
				assertNodesOnce(t, file+" "+v.Name+" ("+string(rendering.Kind)+")", rendering)
			}
			for _, kind := range view.Kinds() {
				if !kind.Supported() {
					continue
				}
				rendering, err := renderer.RenderExposed(tops, kind, "test")
				if err != nil {
					t.Errorf("%s as %s: %v", file, kind, err)
					continue
				}
				assertNodesOnce(t, file+" as "+string(kind), rendering)
			}
		}
	}
	for _, kind := range view.Kinds() {
		if kind.Supported() && !kinds[kind] {
			t.Errorf("no view under testdata renders as %s", kind)
		}
	}
}

// assertNodesOnce fails when two nodes of rendering share an id or one node is placed twice.
func assertNodesOnce(t *testing.T, name string, rendering *view.Rendering) {
	t.Helper()
	ids := map[string]*view.Node{}
	placed := map[*view.Node]bool{}
	var visit func(nodes []*view.Node)
	visit = func(nodes []*view.Node) {
		for _, node := range nodes {
			if node == nil {
				continue
			}
			if placed[node] {
				t.Errorf("%s: node %s (%s) is placed twice", name, node.ID, node.Name)
				continue
			}
			placed[node] = true
			if other, ok := ids[node.ID]; ok {
				t.Errorf("%s: id %s names both %s and %s", name, node.ID, other.Name, node.Name)
			}
			ids[node.ID] = node
			visit(node.Children)
		}
	}
	visit(rendering.Roots)
}

// fixtureRenderer loads files as documents of one model over the standard library.
func fixtureRenderer(t *testing.T, files []string) (*view.Renderer, *symbols.Index) {
	t.Helper()
	idx := libs.NewModelIndex()
	sources := map[string]*source.SourceFile{}
	for _, file := range files {
		content, err := os.ReadFile(filepath.Join("testdata", file))
		if err != nil {
			t.Fatal(err)
		}
		sf := source.New(file, content)
		idx.AddDocument(file, parser.New(sf).ParseFile())
		sources[file] = sf
	}
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	sem := semantics.NewModel(resolver)
	resolver.SetModel(sem)
	text := func(doc string, span source.Span) string {
		if sf, ok := sources[doc]; ok {
			return sf.Text(span)
		}
		return ""
	}
	return view.NewRenderer(sem, resolver, text), idx
}

// fixtureViews is every view declared under scope, in declaration order.
func fixtureViews(scope *symbols.Scope) []*symbols.Symbol {
	var views []*symbols.Symbol
	for _, sym := range scope.AllMembers() {
		if semantics.IsView(sym) {
			views = append(views, sym)
		}
		if sym.Scope != nil {
			views = append(views, fixtureViews(sym.Scope)...)
		}
	}
	return views
}
