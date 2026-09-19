package lower

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Test trigger classification in lowerTransitionMember
func TestTriggerClassification_SignalTrigger(t *testing.T) {
	src := `
		package test {
			state M {
				entry; then start;
				state start;
				state waiting;
				
				succession first start then waiting;
				transition first waiting when powerOn then done;
			}
		}
	`

	file := source.New("test.sysml", []byte(src))
	p := parser.New(file)
	root := p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}

	// Find state usage
	var stateUsage *ast.Usage
	for _, member := range root.Members {
		if membership, ok := member.(*ast.Membership); ok {
			if pkg, ok := membership.Member.(*ast.Package); ok {
				for _, pkgMember := range pkg.Members {
					if pkgMembership, ok := pkgMember.(*ast.Membership); ok {
						if usage, ok := pkgMembership.Member.(*ast.Usage); ok && usage.Kind == ast.UsageState {
							stateUsage = usage
							break
						}
					}
				}
			}
		}
	}

	if stateUsage == nil {
		t.Fatal("no state usage found")
	}

	graph, err := ToStateGraph(stateUsage, nil)
	if err != nil {
		t.Fatalf("ToStateGraph failed: %v", err)
	}

	// Find transition from waiting to done
	var transition *Transition
	var transMember *ast.TransitionMember
	// First find the raw TransitionMember to see what trigger type the parser created
	for _, stateMember := range stateUsage.Members {
		if memb, ok := stateMember.(*ast.Membership); ok {
			if trans, ok := memb.Member.(*ast.TransitionMember); ok {
				if trans.Source != nil && len(trans.Source.Parts) > 0 && trans.Source.Parts[0].Text == "waiting" {
					transMember = trans
					break
				}
			}
		}
	}

	if transMember != nil {
		t.Logf("Raw trigger type before classification: %T", transMember.Trigger)
		if qname, ok := transMember.Trigger.(*ast.QualifiedName); ok {
			t.Logf("QualifiedName parts: %v", qname.Parts)
		}
	}

	for _, trans := range graph.Transitions {
		for _, tr := range trans {
			if stateNode, ok := tr.Source.(*ast.StateNode); ok && stateNode.Name == "waiting" {
				if targetNode, ok := tr.Target.(*ast.StateNode); ok && targetNode.Name == "done" {
					transition = tr
					break
				}
			}
		}
	}

	if transition == nil {
		t.Fatal("transition waiting->done not found")
	}

	// Trigger should be classified as AcceptEvent
	acceptEvent, ok := transition.Trigger.(*ast.AcceptEvent)
	if !ok {
		t.Fatalf("expected AcceptEvent, got %T", transition.Trigger)
	}

	// SignalType should be QualifiedName with "powerOn"
	if acceptEvent.SignalType == nil {
		t.Fatal("AcceptEvent has nil SignalType")
	}

	if len(acceptEvent.SignalType.Parts) != 1 || acceptEvent.SignalType.Parts[0].Text != "powerOn" {
		t.Errorf("expected signal 'powerOn', got %v", acceptEvent.SignalType)
	}
}

func TestTriggerClassification_ChangeEvent(t *testing.T) {
	src := `
		package test {
			state M {
				entry; then start;
				state start;
				state waiting;
				
				succession first start then waiting;
				transition first waiting when x > 5 then done;
			}
		}
	`

	file := source.New("test.sysml", []byte(src))
	p := parser.New(file)
	root := p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}

	// Find state usage
	var stateUsage *ast.Usage
	for _, member := range root.Members {
		if membership, ok := member.(*ast.Membership); ok {
			if pkg, ok := membership.Member.(*ast.Package); ok {
				for _, pkgMember := range pkg.Members {
					if pkgMembership, ok := pkgMember.(*ast.Membership); ok {
						if usage, ok := pkgMembership.Member.(*ast.Usage); ok && usage.Kind == ast.UsageState {
							stateUsage = usage
							break
						}
					}
				}
			}
		}
	}

	if stateUsage == nil {
		t.Fatal("no state usage found")
	}

	graph, err := ToStateGraph(stateUsage, nil)
	if err != nil {
		t.Fatalf("ToStateGraph failed: %v", err)
	}

	// Find transition
	var transition *Transition
	for _, trans := range graph.Transitions {
		for _, t := range trans {
			if stateNode, ok := t.Source.(*ast.StateNode); ok && stateNode.Name == "waiting" {
				transition = t
				break
			}
		}
	}

	if transition == nil {
		t.Fatal("transition not found")
	}

	// Trigger should be classified as ChangeEvent
	changeEvent, ok := transition.Trigger.(*ast.ChangeEvent)
	if !ok {
		t.Fatalf("expected ChangeEvent, got %T", transition.Trigger)
	}

	// Condition should be the expression
	if changeEvent.Condition == nil {
		t.Fatal("ChangeEvent has nil Condition")
	}
}

func TestTriggerClassification_NilCompletion(t *testing.T) {
	src := `
		package test {
			state M {
				entry; then start;
				state start;
				state waiting;
				
				succession first start then waiting;
				transition first waiting then done;
			}
		}
	`

	file := source.New("test.sysml", []byte(src))
	p := parser.New(file)
	root := p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}

	// Find state usage
	var stateUsage *ast.Usage
	for _, member := range root.Members {
		if membership, ok := member.(*ast.Membership); ok {
			if pkg, ok := membership.Member.(*ast.Package); ok {
				for _, pkgMember := range pkg.Members {
					if pkgMembership, ok := pkgMember.(*ast.Membership); ok {
						if usage, ok := pkgMembership.Member.(*ast.Usage); ok && usage.Kind == ast.UsageState {
							stateUsage = usage
							break
						}
					}
				}
			}
		}
	}

	if stateUsage == nil {
		t.Fatal("no state usage found")
	}

	graph, err := ToStateGraph(stateUsage, nil)
	if err != nil {
		t.Fatalf("ToStateGraph failed: %v", err)
	}

	// Find transition
	var transition *Transition
	for _, trans := range graph.Transitions {
		for _, t := range trans {
			if stateNode, ok := t.Source.(*ast.StateNode); ok && stateNode.Name == "waiting" {
				transition = t
				break
			}
		}
	}

	if transition == nil {
		t.Fatal("transition not found")
	}

	// Trigger should be nil (completion transition)
	if transition.Trigger != nil {
		t.Fatalf("expected nil trigger, got %T", transition.Trigger)
	}
}

// A parsed call trigger reaches the graph with its operation and declared
// parameters intact: the runtime matches on both and must not re-derive them.
func TestTriggerClassification_CallTrigger(t *testing.T) {
	src := `
		package test {
			state M {
				entry; then start;
				state start;
				state waiting;

				succession first start then waiting;
				transition first waiting accept setSpeed(value) then done;
			}
		}
	`

	p := parser.New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}

	stateUsage := findStateUsage(root)
	if stateUsage == nil {
		t.Fatal("no state usage found")
	}

	graph, err := ToStateGraph(stateUsage, nil)
	if err != nil {
		t.Fatalf("ToStateGraph failed: %v", err)
	}

	var trigger *ast.CallEvent
	for _, transitions := range graph.Transitions {
		for _, tr := range transitions {
			source, ok := tr.Source.(*ast.StateNode)
			if !ok || source.Name != "waiting" {
				continue
			}
			callEvent, ok := tr.Trigger.(*ast.CallEvent)
			if !ok {
				t.Fatalf("expected CallEvent, got %T", tr.Trigger)
			}
			trigger = callEvent
		}
	}

	if trigger == nil {
		t.Fatal("transition waiting->done not found")
	}
	if got := ast.SimpleName(trigger.Operation); got != "setSpeed" {
		t.Errorf("operation = %q, want setSpeed", got)
	}
	if len(trigger.Parameters) != 1 || trigger.Parameters[0].Text != "value" {
		t.Errorf("parameters = %v, want [value]", trigger.Parameters)
	}
}

// findStateUsage returns the first state usage declared in the first package.
func findStateUsage(root *ast.RootNamespace) *ast.Usage {
	for _, member := range root.Members {
		membership, ok := member.(*ast.Membership)
		if !ok {
			continue
		}
		pkg, ok := membership.Member.(*ast.Package)
		if !ok {
			continue
		}
		for _, pkgMember := range pkg.Members {
			pkgMembership, ok := pkgMember.(*ast.Membership)
			if !ok {
				continue
			}
			if usage, ok := pkgMembership.Member.(*ast.Usage); ok && usage.Kind == ast.UsageState {
				return usage
			}
		}
	}
	return nil
}

func TestTriggerClassification_AlreadyTyped(t *testing.T) {
	// Test that already-typed triggers pass through unchanged
	timeEvent := &ast.TimeEvent{Duration: &ast.LiteralInteger{Value: "5"}}
	result := classifyTrigger(timeEvent)

	if result != timeEvent {
		t.Error("TimeEvent should pass through unchanged")
	}

	changeEvent := &ast.ChangeEvent{Condition: &ast.LiteralBool{Value: true}}
	result = classifyTrigger(changeEvent)

	if result != changeEvent {
		t.Error("ChangeEvent should pass through unchanged")
	}

	acceptEvent := &ast.AcceptEvent{SignalType: &ast.QualifiedName{}}
	result = classifyTrigger(acceptEvent)

	if result != acceptEvent {
		t.Error("AcceptEvent should pass through unchanged")
	}

	callEvent := &ast.CallEvent{Operation: &ast.QualifiedName{}}
	result = classifyTrigger(callEvent)

	if result != callEvent {
		t.Error("CallEvent should pass through unchanged")
	}
}
