package parser

import (
	"fmt"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A direction before any usage kind a behavior body admits declares a directed
// usage of that kind (SysML.xtext RefPrefix via OccurrenceUsagePrefix), not a
// parameter named by the kind keyword.
func TestDirectedUsageKindsInBehaviorBody(t *testing.T) {
	cases := []struct {
		member string
		kind   ast.UsageKind
		name   string
		dir    ast.FeatureDirection
	}{
		{"in concern c : C;", ast.UsageConcern, "c", ast.DirIn},
		{"out requirement r : R { subject s : S; }", ast.UsageRequirement, "r", ast.DirOut},
		{"out use case u[*] : U;", ast.UsageUseCase, "u", ast.DirOut},
		{"inout verification v : V;", ast.UsageVerificationCase, "v", ast.DirInOut},
		{"in analysis a : A;", ast.UsageAnalysisCase, "a", ast.DirIn},
		{"in case k : K;", ast.UsageCase, "k", ast.DirIn},
		{"in constraint q : Q;", ast.UsageConstraint, "q", ast.DirIn},
		{"in state s;", ast.UsageState, "s", ast.DirIn},
		{"in view w : W;", ast.UsageView, "w", ast.DirIn},
		{"in viewpoint p : P;", ast.UsageViewpoint, "p", ast.DirIn},
		{"in rendering g : G;", ast.UsageRendering, "g", ast.DirIn},
		{"in enum e : E;", ast.UsageEnumeration, "e", ast.DirIn},
		{"out ref concern c : C;", ast.UsageConcern, "c", ast.DirOut},
		{"out derived use case u : U;", ast.UsageUseCase, "u", ast.DirOut},
		{"in item i : I;", ast.UsageItem, "i", ast.DirIn},
	}
	for _, body := range []string{"action def A { %s }", "calc def F { %s }", "action a { %s }"} {
		for _, c := range cases {
			src := fmt.Sprintf(body, c.member)
			t.Run(src, func(t *testing.T) {
				p := New(source.New("test.sysml", []byte(src)))
				root := p.ParseFile()
				if len(p.Diagnostics) != 0 {
					t.Fatalf("diagnostics = %v, want none", p.Diagnostics)
				}
				u := firstUsageWithDirection(root)
				if u == nil {
					t.Fatal("no directed usage parsed")
				}
				if u.Kind != c.kind || u.Ident.Name != c.name || u.Direction != c.dir {
					t.Errorf("got %v %q %v, want %v %q %v", u.Kind, u.Ident.Name, u.Direction, c.kind, c.name, c.dir)
				}
			})
		}
	}
}

func firstUsageWithDirection(root ast.Node) *ast.Usage {
	var found *ast.Usage
	ast.Inspect(root, func(n ast.Node) bool {
		if found != nil {
			return false
		}
		if u, ok := n.(*ast.Usage); ok && u.Direction != ast.DirNone {
			found = u
			return false
		}
		return true
	})
	return found
}
