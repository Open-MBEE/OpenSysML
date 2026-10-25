package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A `ref` before the two-word `use case` kind marks the usage a reference
// wherever it is written, in a body as at the top level (SysML.xtext UsagePrefix).
func TestRefUseCaseInBodyIsReference(t *testing.T) {
	src := `package P {
		ref use case top;
		part def A { ref use case nested; use case owned; }
		action act { private ref use case acted : U; }
	}`
	file := New(source.New("test", []byte(src))).ParseFile()
	got := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		if u, ok := n.(*ast.Usage); ok && u.Kind == ast.UsageUseCase {
			got[u.Ident.Name] = u.IsReference
		}
		return true
	})
	want := map[string]bool{"top": true, "nested": true, "owned": false, "acted": true}
	for name, ref := range want {
		if isRef, ok := got[name]; !ok || isRef != ref {
			t.Errorf("use case %s: IsReference = %v (declared %v), want %v", name, isRef, ok, ref)
		}
	}
}
