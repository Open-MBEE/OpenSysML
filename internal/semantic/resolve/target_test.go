package resolve

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// resolveIndex parses and indexes src without resolving it, so that a test can
// drive resolution itself.
func resolveIndex(t *testing.T, src string) (*Resolver, *symbols.Scope) {
	t.Helper()
	const name = "t.sysml"
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc(name, root)
	return New(idx), idx.DocumentRoot(name)
}

func TestResolveTargetFollowsFeatureChain(t *testing.T) {
	r, root := resolveIndex(t, `package P {
		action providePower { action generateTorque; }
		part torqueGenerator { perform providePower.generateTorque; }
	}`)

	pkg, _ := root.LookupLocal("P")
	generator, _ := pkg.Scope.LookupLocal("torqueGenerator")
	perform, ok := generator.Scope.LookupLocal("generateTorque")
	if !ok {
		t.Fatalf("perform statement did not bind generateTorque")
	}
	usage, ok := perform.Decl.(*ast.Usage)
	if !ok {
		t.Fatalf("perform decl = %T, want *ast.Usage", perform.Decl)
	}

	sym, ok := r.ResolveTarget(pkg.Scope, usage.Relationships[0].Target)
	if !ok {
		t.Fatalf("ResolveTarget(providePower.generateTorque) failed")
	}
	action, _ := pkg.Scope.LookupLocal("providePower")
	want, _ := action.Scope.LookupLocal("generateTorque")
	if sym != want {
		t.Fatalf("ResolveTarget = %v, want the action's generateTorque", sym)
	}
}

// The feature a perform statement declares carries the referenced feature's
// name, so the reference itself must resolve outside that binding.
func TestReferenceTargetSkipsSelfBinding(t *testing.T) {
	r, root := resolveIndex(t, `package P {
		action providePower;
		part vehicle { perform providePower; }
	}`)

	pkg, _ := root.LookupLocal("P")
	vehicle, _ := pkg.Scope.LookupLocal("vehicle")
	perform, _ := vehicle.Scope.LookupLocal("providePower")
	usage := perform.Decl.(*ast.Usage)
	target := usage.Relationships[0].Target

	sym, ok := r.ResolveReferenceTarget(vehicle.Scope, usage, target)
	action, _ := pkg.Scope.LookupLocal("providePower")
	if !ok || sym != action {
		t.Fatalf("ResolveReferenceTarget = %v, want the action providePower", sym)
	}
}

func TestResolveTargetUnknown(t *testing.T) {
	r, root := resolveIndex(t, "package P { part p; }")
	pkg, _ := root.LookupLocal("P")

	if _, ok := r.ResolveTarget(pkg.Scope, nil); ok {
		t.Fatalf("nil target must not resolve")
	}
	missing := &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "nope"}}}
	if _, ok := r.ResolveTarget(pkg.Scope, missing); ok {
		t.Fatalf("unknown target must not resolve")
	}
}
