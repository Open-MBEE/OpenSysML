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

func TestViaRouteMembersDoNotResolveOutward(t *testing.T) {
	const name = "via.sysml"
	src := `package P {
		item def Go;
		part def Owner {
			port p;
		}
		part def Scope {
			part other {
				port p;
			}
			state def Life {
				in ref context : Owner;
				state ready;
				state done;
				transition first ready accept Go via context.p then done;
				transition first done accept Go via context.other.p then ready;
			}
		}
	}`
	p := parser.New(source.New(name, []byte(src)))
	rootNode := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc(name, rootNode)
	r := New(idx)
	r.ResolveDocument(name, rootNode)

	root := idx.DocumentRoot(name)
	pkg, _ := root.LookupLocal("P")
	if pkg == nil {
		t.Fatal("fixture is missing package P")
	}
	owner, _ := pkg.Scope.LookupLocal("Owner")
	scope, _ := pkg.Scope.LookupLocal("Scope")
	if owner == nil || scope == nil {
		t.Fatal("fixture is missing Owner or Scope")
	}
	life, _ := scope.Scope.LookupLocal("Life")
	if life == nil {
		t.Fatal("fixture is missing Scope::Life")
	}
	context, _ := life.Scope.LookupLocal("context")
	if context == nil {
		t.Fatal("fixture is missing Life.context")
	}
	ownerPort, _ := owner.Scope.LookupLocal("p")
	outerOther, _ := scope.Scope.LookupLocal("other")
	if outerOther == nil {
		t.Fatal("fixture is missing Scope.other")
	}
	outerPort, _ := outerOther.Scope.LookupLocal("p")
	if ownerPort == nil || outerPort == nil {
		t.Fatal("fixture must contain both Owner.p and the unrelated Scope.other.p")
	}

	lifeDef, ok := life.Scope.Node().(*ast.Definition)
	if !ok {
		t.Fatalf("Life scope node = %T, want *ast.Definition", life.Scope.Node())
	}
	var positive, negative *ast.QualifiedName
	addRoute := func(qn *ast.QualifiedName) {
		if len(qn.Parts) == 2 && qn.Parts[0].Text == "context" && qn.Parts[1].Text == "p" {
			positive = qn
		}
		if len(qn.Parts) == 3 && qn.Parts[0].Text == "context" &&
			qn.Parts[1].Text == "other" && qn.Parts[2].Text == "p" {
			negative = qn
		}
	}
	for _, member := range lifeDef.Members {
		switch m := member.(type) {
		case *ast.TransitionMember:
			if m.Via != nil {
				addRoute(m.Via)
			}
		case *ast.Membership:
			if transition, ok := m.Member.(*ast.TransitionMember); ok && transition.Via != nil {
				addRoute(transition.Via)
			}
		}
	}

	if positive == nil {
		t.Fatal("fixture is missing via context.p")
	}
	if got, ok := r.PartSymbol(positive, 0); !ok || got != context {
		t.Errorf("context.p head = %v, want context parameter %v", got, context)
	}
	if got, ok := r.PartSymbol(positive, 1); !ok || got != ownerPort {
		t.Errorf("context.p member = %v, want Owner.p %v", got, ownerPort)
	}

	if negative == nil {
		t.Fatal("fixture is missing via context.other.p")
	}
	if got, ok := r.PartSymbol(negative, 0); !ok || got != context {
		t.Errorf("context.other.p head = %v, want context parameter %v", got, context)
	}
	if got, ok := r.PartSymbol(negative, 1); ok {
		t.Errorf("context.other.p resolved `other` as %v; it must not bind Scope.other", got)
	}
	if got, ok := r.PartSymbol(negative, 2); ok {
		t.Errorf("context.other.p resolved `p` as %v; it must stay unresolved", got)
	}
}

func TestSendViaBodyRouteMembersDoNotResolveOutward(t *testing.T) {
	const name = "send-via-body.sysml"
	src := `package P {
		attribute def Integer;
		item def Go {
			in x : Integer;
		}
		part def Owner {
			port p;
		}
		part def Scope {
			part other {
				port p;
			}
			action def A {
				in ref context : Owner;
				action sendPositive send new Go(1) via context.p { in x = 2; }
				action sendNegative send new Go(1) via context.other.p { in x = 2; }
			}
		}
	}`
	p := parser.New(source.New(name, []byte(src)))
	rootNode := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc(name, rootNode)
	r := New(idx)
	r.ResolveDocument(name, rootNode)

	root := idx.DocumentRoot(name)
	pkg, _ := root.LookupLocal("P")
	owner, _ := pkg.Scope.LookupLocal("Owner")
	scope, _ := pkg.Scope.LookupLocal("Scope")
	if owner == nil || scope == nil {
		t.Fatal("fixture is missing Owner or Scope")
	}
	action, _ := scope.Scope.LookupLocal("A")
	if action == nil {
		t.Fatal("fixture is missing Scope::A")
	}
	context, _ := action.Scope.LookupLocal("context")
	if context == nil {
		t.Fatal("fixture is missing A.context")
	}
	ownerPort, _ := owner.Scope.LookupLocal("p")
	outerOther, _ := scope.Scope.LookupLocal("other")
	if outerOther == nil {
		t.Fatal("fixture is missing Scope.other")
	}
	outerPort, _ := outerOther.Scope.LookupLocal("p")
	if ownerPort == nil || outerPort == nil {
		t.Fatal("fixture must contain both Owner.p and the unrelated Scope.other.p")
	}

	var positive, negative *ast.QualifiedName
	for _, ref := range References(rootNode, root) {
		if !ref.Via || ref.QN == nil {
			continue
		}
		parts := ref.QN.Parts
		if len(parts) == 2 && parts[0].Text == "context" && parts[1].Text == "p" {
			positive = ref.QN
		}
		if len(parts) == 3 && parts[0].Text == "context" &&
			parts[1].Text == "other" && parts[2].Text == "p" {
			negative = ref.QN
		}
	}
	if positive == nil || negative == nil {
		t.Fatalf("send via references = %v, want context.p and context.other.p", References(rootNode, root))
	}
	if got, ok := r.PartSymbol(positive, 0); !ok || got != context {
		t.Errorf("context.p head = %v, want context parameter %v", got, context)
	}
	if got, ok := r.PartSymbol(positive, 1); !ok || got != ownerPort {
		t.Errorf("context.p member = %v, want Owner.p %v", got, ownerPort)
	}
	if got, ok := r.PartSymbol(negative, 0); !ok || got != context {
		t.Errorf("context.other.p head = %v, want context parameter %v", got, context)
	}
	if got, ok := r.PartSymbol(negative, 1); ok {
		t.Errorf("context.other.p resolved `other` as %v; it must not bind Scope.other", got)
	}
	if got, ok := r.PartSymbol(negative, 2); ok {
		t.Errorf("context.other.p resolved `p` as %v; it must stay unresolved", got)
	}
}
