package lower

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A node answers to its name, the empty unrestricted name included, and a usage to its short name too.
func TestNodeAnswersTo(t *testing.T) {
	written := source.Span{Offset: 1, Len: 2}
	usage := func(id ast.Identification) *ast.Usage { return &ast.Usage{Ident: id} }
	for _, c := range []struct {
		label string
		node  ast.Node
		name  string
		want  bool
	}{
		{"named usage", usage(ast.Identification{Name: "a", NameSpan: written}), "a", true},
		{"other name", usage(ast.Identification{Name: "a", NameSpan: written}), "b", false},
		{"short name", usage(ast.Identification{Name: "a", NameSpan: written, ShortName: "s", ShortNameSpan: written}), "s", true},
		{"short name alone", usage(ast.Identification{ShortName: "s", ShortNameSpan: written}), "s", true},
		{"unnamed usage", usage(ast.Identification{}), "", false},
		{"empty unrestricted name", usage(ast.Identification{NameSpan: written}), "", true},
		{"fork", &ast.ForkNode{Name: "f"}, "f", true},
		{"unnamed fork", &ast.ForkNode{}, "", false},
		{"empty-named fork", &ast.ForkNode{NameSpan: written}, "", true},
		{"final", &ast.FinalNode{}, "done", true},
		{"final by no name", &ast.FinalNode{}, "", false},
	} {
		if got := nodeAnswersTo(c.node, c.name); got != c.want {
			t.Errorf("%s: nodeAnswersTo(%q) = %v, want %v", c.label, c.name, got, c.want)
		}
		name, named := nodeName(c.node)
		if name != getNodeName(c.node) || named != nodeHasName(c.node) {
			t.Errorf("%s: nodeName = %q, %v; getNodeName %q, nodeHasName %v", c.label, name, named, getNodeName(c.node), nodeHasName(c.node))
		}
	}
}
