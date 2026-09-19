package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// untyped is an ArgumentTyper that compares by value, as the checker's does.
type untyped struct{ tag int }

func (untyped) InvocationArguments(_ *symbols.Scope, e *ast.InvocationExpr) []Argument {
	return untypedArguments(e)
}

// Installing the typing already in place keeps the selections made under it;
// a different typing drops them.
func TestSetArgumentTyperKeepsSelectionsUnderTheSameTyping(t *testing.T) {
	m := NewModel(nil)
	m.SetArgumentTyper(untyped{1})
	m.invocations[invocationKey{}] = &InvocationSelection{}
	m.SetArgumentTyper(untyped{1})
	if m.MemoSize() != 1 {
		t.Fatal("reinstalling the same typing dropped the selections")
	}
	m.SetArgumentTyper(untyped{2})
	if m.MemoSize() != 0 {
		t.Fatal("a different typing kept the selections made under the old one")
	}
}

// sliced is an ArgumentTyper whose values cannot be compared.
type sliced []int

func (sliced) InvocationArguments(_ *symbols.Scope, e *ast.InvocationExpr) []Argument {
	return untypedArguments(e)
}

// A typer that cannot be compared installs without panicking and, since it
// cannot be shown to be the typing in place, drops the selections.
func TestSetArgumentTyperTreatsAnIncomparableTyperAsNew(t *testing.T) {
	m := NewModel(nil)
	m.SetArgumentTyper(sliced(nil))
	m.invocations[invocationKey{}] = &InvocationSelection{}
	m.SetArgumentTyper(sliced(nil))
	if m.MemoSize() != 0 {
		t.Fatal("an incomparable typer kept selections it could not be shown to have made")
	}
	m.invocations[invocationKey{}] = &InvocationSelection{}
	m.SetArgumentTyper(untyped{1})
	if m.MemoSize() != 0 {
		t.Fatal("a typer of another type kept the selections")
	}
}
