package runtime

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// transitionPayload reads `T.d`, a transition's accepted payload, from within a
// state machine's behavior: the value T's trigger bound when T is the transition
// being taken, and nothing — a transition not taken performs nothing — otherwise.
// It declines chains whose base is not a transition or read outside a machine.
func (ec *EvalContext) transitionPayload(base ast.Node, parts []ast.NameSegment) (Value, bool, error) {
	f, ok := ec.machineFiring()
	if !ok {
		return Value{}, false, nil
	}
	sym, ok := ec.chainBaseSymbol(base)
	if !ok {
		return Value{}, false, nil
	}
	trans, ok := sym.Decl.(*ast.TransitionMember)
	if !ok {
		return Value{}, false, nil
	}
	if f.taken == nil || f.taken.Decl != ast.Node(trans) {
		return nullValue(), true, nil
	}
	name := parts[0].Text
	if !slices.Contains(f.taken.Accepted, name) {
		return Value{}, false, nil
	}
	value, ok := f.payload[name]
	if !ok {
		return Value{}, true, &NoValueError{Feature: sym.Name + "." + name, Symbol: sym}
	}
	if len(parts) == 1 {
		return value, true, nil
	}
	rest, err := ec.chainMemberValue(value, parts[1:], name)
	return rest, true, err
}

// machineFiring is the state machine firing the innermost frame read within one
// reports, if the evaluation is a machine behavior's.
func (ec *EvalContext) machineFiring() (*firing, bool) {
	for i := len(ec.frames) - 1; i >= 0; i-- {
		if f := ec.frames[i].firing; f != nil {
			return f, true
		}
	}
	return nil, false
}
