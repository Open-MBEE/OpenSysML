package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/edit"
	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
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

// sequenceContext parses content as one document over the library index and
// returns a context whose ExecuteAction can perform its actions.
func sequenceContext(t *testing.T, content string) (*Context, *symbols.Index, *symbols.Scope) {
	t.Helper()
	sf := source.New("sequence.sysml", []byte(content))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument(sf.Name(), root)
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	return NewContext(typedModel(model, resolver), 10000), idx, idx.DocumentRoot(sf.Name())
}

// Executing the Editor-authored ToastBread gives the same outcome and trace as
// executing the directly written notation of the same body.
func TestSequenceAuthoredActionsExecuteLikeWrittenOnes(t *testing.T) {
	sf := source.New("toaster.sysml", []byte(sequenceToasterStart))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument(sf.Name(), root)
	model := edit.Model{
		Source:   sf,
		Root:     root,
		Index:    idx,
		SemDiags: passes.Analyze(sf.Name(), root, nil, idx),
		NewIndex: func() *symbols.Index { return libs.NewModelIndex() },
	}
	result, err := edit.Apply(model, []edit.Operation{
		edit.AddFirst("ToasterDemo::ApplyHeat", "start"),
		edit.AddThenMember("ToasterDemo::ApplyHeat", "action", "generateHeat", "GenerateHeat"),
		{Kind: edit.OpAddMember, Owner: "ToasterDemo::ApplyHeat::generateHeat", MemberKind: "", MemberName: "energyIn", Value: "ApplyHeat::energy", Direction: "in"},
		edit.AddThen("ToasterDemo::ApplyHeat", "done"),
		edit.AddFirst("ToasterDemo::ToastBread", "start"),
		edit.AddThenMember("ToasterDemo::ToastBread", "action", "applyHeat", "ApplyHeat"),
		{Kind: edit.OpAddMember, Owner: "ToasterDemo::ToastBread::applyHeat", MemberKind: "", MemberName: "bread", Value: "ToastBread::bread", Direction: "in"},
		edit.AddThen("ToasterDemo::ToastBread", "done"),
	})
	if err != nil {
		t.Fatalf("edit.Apply: %v", err)
	}

	// The written spelling of the body the ops build.
	written := `package ToasterDemo {
    private import ISQ::*;
    item def Bread;
    item def Toast;
    action def GenerateHeat { in energyIn : ISQ::EnergyValue[0..*]; }
    action def ApplyHeat {
        in bread : Bread;
        in energy : ISQ::EnergyValue[0..*];
        out toast : Toast;
        first start;
        then action generateHeat : GenerateHeat {
            in energyIn = ApplyHeat::energy;
        }
        then done;
    }
    action def ToastBread {
        in bread : Bread;
        out toast : Toast;
        first start;
        then action applyHeat : ApplyHeat {
            in bread = ToastBread::bread;
        }
        then done;
    }
}
`
	if string(result.Content) != written {
		t.Fatalf("edited text\n%s\nwant\n%s", result.Content, written)
	}

	runToastBread := func(t *testing.T, content string) (Outcome, string) {
		t.Helper()
		ctx, idx, scope := sequenceContext(t, content)
		trace := NewTraceRecorder()
		ctx.SetTrace(trace)
		actionSym := namedOrFoundSymbol(t, idx, "ToasterDemo::ToastBread", scope, ast.DefAction, ast.UsageAction)
		outputs, err := ctx.ExecuteAction(actionSym)
		if err != nil {
			t.Fatalf("ExecuteAction: %v", err)
		}
		return ctx.ActionOutcome(outputs), trace.String()
	}

	editedOutcome, editedTrace := runToastBread(t, string(result.Content))
	writtenOutcome, writtenTrace := runToastBread(t, written)
	if editedTrace != writtenTrace {
		t.Fatalf("traces differ\n=== EDITED ===\n%s\n=== WRITTEN ===\n%s", editedTrace, writtenTrace)
	}
	if got, want := editedOutcome.String(), writtenOutcome.String(); got != want {
		t.Fatalf("outcome = %v, want %v", got, want)
	}
}
