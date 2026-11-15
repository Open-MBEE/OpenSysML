package lower

import (
	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// withExprs is stmt with each expression it states interned in exprs, in the scope
// the statement evaluates it in; the statements of its blocks were interned as lowered.
func withExprs(stmt Statement, exprs *ExprTable) Statement {
	if exprs == nil {
		return stmt
	}
	id := func(have *program.ExprID, node ast.Node, scope *symbols.Scope) {
		if *have == 0 {
			*have = exprs.Intern(node, scope)
		}
	}
	switch s := stmt.(type) {
	case Assign:
		id(&s.ValueID, s.Value, s.Scope)
		if s.Chain != nil {
			id(&s.Chain.BaseID, s.Chain.Base, s.Scope)
		}
		return s
	case Declare:
		id(&s.ValueID, s.Value, s.Scope)
		return s
	case Return:
		id(&s.ValueID, s.Value, s.Scope)
		return s
	case If:
		id(&s.ConditionID, s.Condition, s.Scope)
		return s
	case Loop:
		// A loop's conditions read what its body declares; its collection is read before it.
		id(&s.ConditionID, s.Condition, s.Body.Scope)
		id(&s.UntilID, s.Until, s.Body.Scope)
		id(&s.CollectionID, s.Collection, s.Scope)
		return s
	case Send:
		id(&s.MessageID, s.Message, s.Scope)
		id(&s.TargetExprID, s.TargetExpr, s.Scope)
		id(&s.ReceiverExprID, s.ReceiverExpr, s.Scope)
		return s
	case Effect:
		id(&s.TargetExprID, s.TargetExpr, s.Scope)
		return s
	}
	return stmt
}

// withBindingExprs is binding with the other end it states interned in exprs, in the
// scope the binding was written in; an end at a node's pin is read from that pin.
func withBindingExprs(binding PinBinding, exprs *ExprTable) PinBinding {
	if exprs == nil || binding.OtherNode != nil {
		return binding
	}
	binding.OtherID = exprs.Intern(binding.Other, binding.Scope)
	if binding.OtherChain != nil {
		chain := *binding.OtherChain
		chain.BaseID = exprs.Intern(chain.Base, binding.Scope)
		binding.OtherChain = &chain
	}
	return binding
}
