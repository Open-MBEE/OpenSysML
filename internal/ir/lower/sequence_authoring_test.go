package lower_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/edit"
	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

const sequenceToasterStart = `package ToasterDemo {
    private import ISQ::*;
    item def Bread;
    item def Toast;
    action def GenerateHeat { in energyIn : ISQ::EnergyValue[0..*]; }
    action def ApplyHeat {
        in bread : Bread;
        in energy : ISQ::EnergyValue[0..*];
        out toast : Toast;
    }
    action def ToastBread {
        in bread : Bread;
        out toast : Toast;
    }
}
`

// sequenceEditModel loads src as the document the edits apply over, with the
// library index `first`/`then` refs like start and done resolve through.
func sequenceEditModel(t *testing.T, src string) edit.Model {
	t.Helper()
	sf := source.New("toaster.sysml", []byte(src))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument(sf.Name(), root)
	return edit.Model{
		Source:   sf,
		Root:     root,
		Index:    idx,
		SemDiags: passes.Analyze(sf.Name(), root, nil, idx),
		NewIndex: func() *symbols.Index { return libs.NewModelIndex() },
	}
}

// sequenceMember unwraps a membership to the element it declares.
func sequenceMember(member ast.Node) ast.Node {
	if m, ok := member.(*ast.Membership); ok {
		return m.Member
	}
	return member
}

// actionDefOf reparses content and returns the action definition qname names.
func actionDefOf(t *testing.T, content, qname string) *ast.Definition {
	t.Helper()
	p := parser.New(source.New("edited.sysml", []byte(content)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("edited text does not parse: %v", p.Diagnostics)
	}
	found := findActionDef(root.Members, qname)
	if found == nil {
		t.Fatalf("no action def named %q in edited text", qname)
	}
	return found
}

// memberChildren returns the members a member declares below itself.
func memberChildren(member ast.Node) []ast.Node {
	switch n := sequenceMember(member).(type) {
	case *ast.Package:
		return n.Members
	case *ast.Namespace:
		return n.Members
	default:
		return ast.DeclMembers(n)
	}
}

// findActionDef returns the action definition qname names among members.
func findActionDef(members []ast.Node, qname string) *ast.Definition {
	for _, member := range members {
		if def, ok := sequenceMember(member).(*ast.Definition); ok &&
			def.Kind == ast.DefAction && def.Ident.Name == qname {
			return def
		}
		if def := findActionDef(memberChildren(member), qname); def != nil {
			return def
		}
	}
	return nil
}

// nodeOrder returns each graph node's name, the implicit markers by kind.
func nodeOrder(graph *lower.ActionGraph) []string {
	names := make([]string, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		switch n := node.(type) {
		case *ast.Usage:
			name, _ := ast.EffectiveName(n)
			names = append(names, name)
		case *ast.InitialNode:
			names = append(names, "start")
		case *ast.FinalNode:
			names = append(names, "done")
		default:
			names = append(names, fmt.Sprintf("%T", n))
		}
	}
	return names
}

// The first/then members the edits author lower as the written notation does:
// start → generateHeat/applyHeat → done, in the order the `then`s were placed.
func TestSequenceAuthoredMembersLowerInOrder(t *testing.T) {
	model := sequenceEditModel(t, sequenceToasterStart)
	result, err := edit.Apply(model, []edit.Operation{
		edit.AddFirst("ToasterDemo::ApplyHeat", "start"),
		edit.AddThenMember("ToasterDemo::ApplyHeat", "action", "generateHeat", "GenerateHeat"),
		edit.AddThen("ToasterDemo::ApplyHeat", "done"),
		edit.AddFirst("ToasterDemo::ToastBread", "start"),
		edit.AddThenMember("ToasterDemo::ToastBread", "action", "applyHeat", "ApplyHeat"),
		edit.AddThen("ToasterDemo::ToastBread", "done"),
	})
	if err != nil {
		t.Fatalf("edit.Apply: %v", err)
	}

	applyHeat := actionDefOf(t, string(result.Content), "ApplyHeat")
	graph, err := lower.ToActionGraph(applyHeat, nil)
	if err != nil {
		t.Fatalf("ToActionGraph ApplyHeat: %v", err)
	}
	if got, want := nodeOrder(graph), []string{"start", "generateHeat", "done"}; !slices.Equal(got, want) {
		t.Fatalf("ApplyHeat node order = %v, want %v", got, want)
	}

	toastBread := actionDefOf(t, string(result.Content), "ToastBread")
	graph, err = lower.ToActionGraph(toastBread, nil)
	if err != nil {
		t.Fatalf("ToActionGraph ToastBread: %v", err)
	}
	if got, want := nodeOrder(graph), []string{"start", "applyHeat", "done"}; !slices.Equal(got, want) {
		t.Fatalf("ToastBread node order = %v, want %v", got, want)
	}
}

// A `then` placed after an earlier member sequences between it and what
// followed it, as the chain-insertion rule writes it.
func TestSequenceAfterInsertionLowersBetween(t *testing.T) {
	model := sequenceEditModel(t, `action def A {
    first start;
    action b;
    then done;
}
`)
	result, err := edit.Apply(model, []edit.Operation{
		{Kind: edit.OpAddSequence, Owner: "A", SequenceKeyword: "then", SequenceRef: "b", After: "b"},
	})
	if err != nil {
		t.Fatalf("edit.Apply: %v", err)
	}
	def := actionDefOf(t, string(result.Content), "A")
	graph, err := lower.ToActionGraph(def, nil)
	if err != nil {
		t.Fatalf("ToActionGraph: %v", err)
	}
	if got, want := nodeOrder(graph), []string{"start", "b", "done"}; !slices.Equal(got, want) {
		t.Fatalf("node order = %v, want %v", got, want)
	}
}

func TestActionBodyStatementsLowerRecursively(t *testing.T) {
	content := `package Demo {
    private import ScalarValues::*;
    attribute def Signal { attribute value : Integer; }
    action def A {
        out result : Integer = 0;
        first start;
        then send new Signal(value = 7) to self;
        then accept msg : Signal;
        then assign result := msg.value;
        then if result == 0 {
            assign result := 1;
        } else {
            assign result := 2;
        }
        then while result < 3 {
            assign result := result + 1;
        } until result == 3;
        then loop {
            assign result := result + 1;
        } until result == 4;
        then for i in (1, 2, 3) {
            assign result := result + i;
        }
        then terminate;
    }
}
`
	action := actionDefOf(t, content, "A")
	graph, err := lower.ToActionGraph(action, nil)
	if err != nil {
		t.Fatalf("ToActionGraph: %v", err)
	}
	counts := map[string]int{}
	var visitGraph func(*lower.ActionGraph)
	var visitStatement func(lower.Statement)
	visitStatement = func(statement lower.Statement) {
		switch statement := statement.(type) {
		case lower.Send:
			counts["send"]++
		case lower.Assign:
			counts["assign"]++
		case lower.Effect:
			if statement.Kind == lower.EffectTerminate {
				counts["terminate"]++
			}
		case lower.If:
			counts["if"]++
			for _, child := range statement.Then.Statements {
				visitStatement(child)
			}
			if statement.Else != nil {
				for _, child := range statement.Else.Statements {
					visitStatement(child)
				}
			}
		case lower.Loop:
			switch statement.Kind {
			case ast.LoopWhile:
				counts["while"]++
			case ast.LoopUntil:
				counts["loop"]++
			case ast.LoopFor:
				counts["for"]++
			}
			for _, child := range statement.Body.Statements {
				visitStatement(child)
			}
		case lower.Block:
			for _, child := range statement.Statements {
				visitStatement(child)
			}
			visitGraph(statement.Graph)
		}
	}
	visitGraph = func(graph *lower.ActionGraph) {
		if graph == nil {
			return
		}
		counts["accept"] += len(graph.Accepts)
		for _, statements := range graph.Bodies {
			for _, statement := range statements {
				visitStatement(statement)
			}
		}
		for _, subflow := range graph.Subflows {
			if subflow != nil {
				visitGraph(subflow.Graph)
			}
		}
	}
	visitGraph(graph)
	for kind, want := range map[string]int{
		"send": 1, "accept": 1, "assign": 6, "if": 1,
		"while": 1, "loop": 1, "for": 1, "terminate": 1,
	} {
		if got := counts[kind]; got != want {
			t.Errorf("lowered %s statements = %d, want %d (all: %v)", kind, got, want, counts)
		}
	}
}
