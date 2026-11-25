package resolve_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A `first s;` marker binds no inherited action its body redefines under another
// name (KerML 8.3.3.1.10), and binds the unnamed action redefining it.
func TestStartMarkerDoesNotBindARedefinedInheritedAction(t *testing.T) {
	const src = `package P {
	action def Base { action s; action u; }
	action def Inherits :> Base { first s; }
	action def Renames :> Base { action t :>> s; first s; }
	action def Restates :> Base { action :>> s; first s; }
}`
	walk, root, _ := resolvedDoc(t, src)
	bound := map[string]ast.Node{}
	var base ast.Node
	for _, def := range unwrapMember(root.Members[0]).(*ast.Package).Members {
		definition := unwrapMember(def).(*ast.Definition)
		for _, member := range definition.Members {
			switch n := unwrapMember(member).(type) {
			case *ast.InitialNode:
				if sym, ok := walk.InitialSymbol(n); ok {
					bound[definition.Ident.Name] = sym.Decl
				}
			case *ast.Usage:
				switch {
				case definition.Ident.Name == "Base" && n.Ident.Name == "s":
					base = n
				case definition.Ident.Name == "Restates":
					bound["restating"] = n
				}
			}
		}
	}
	if bound["Inherits"] != base {
		t.Errorf("`first s;` under the inherited s binds %v, want Base::s", bound["Inherits"])
	}
	if got, ok := bound["Renames"]; ok {
		t.Errorf("`first s;` beside `action t :>> s` binds %v, want nothing: s is not inherited", got)
	}
	if bound["Restates"] == nil || bound["Restates"] != bound["restating"] {
		t.Errorf("`first s;` beside `action :>> s` binds %v, want the redefining action", bound["Restates"])
	}
}
