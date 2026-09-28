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
	var found *ast.Definition
	var membersOf func(member ast.Node) []ast.Node
	membersOf = func(member ast.Node) []ast.Node {
		switch n := sequenceMember(member).(type) {
		case *ast.Package:
			return n.Members
		case *ast.Namespace:
			return n.Members
		default:
			return ast.DeclMembers(n)
		}
	}
	var walk func(members []ast.Node)
	walk = func(members []ast.Node) {
		for _, member := range members {
			if def, ok := sequenceMember(member).(*ast.Definition); ok && def.Kind == ast.DefAction {
				if def.Ident.Name == qname {
					found = def
				}
			}
			walk(membersOf(member))
		}
	}
	walk(root.Members)
	if found == nil {
		t.Fatalf("no action def named %q in edited text", qname)
	}
	return found
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
