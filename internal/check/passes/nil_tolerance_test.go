package passes

import (
	"fmt"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Every registered pass, run alone or through the registry, reports nothing
// rather than panicking for a nil context, a context with no index, and a nil root.
func TestRegisteredPassesTolerateNilArguments(t *testing.T) {
	root := &ast.RootNamespace{}
	cases := []struct {
		name string
		ctx  func() *Context
		root *ast.RootNamespace
	}{
		{"nil context", func() *Context { return nil }, root},
		{"nil index", func() *Context { return NewContext("t.sysml", nil, nil) }, root},
		{"nil root", func() *Context { return NewContext("t.sysml", nil, nil) }, nil},
	}
	for _, c := range cases {
		for _, pass := range DefaultRegistry().Passes() {
			t.Run(fmt.Sprintf("%s/%T", c.name, pass), func(t *testing.T) {
				if diags := pass.Run(c.ctx(), "t.sysml", c.root); len(diags) != 0 {
					t.Errorf("diagnostics = %v, want none", diags)
				}
			})
		}
		t.Run(c.name+"/registry", func(t *testing.T) {
			if diags := DefaultRegistry().Run(c.ctx(), "t.sysml", c.root); len(diags) != 0 {
				t.Errorf("diagnostics = %v, want none", diags)
			}
		})
	}
}
