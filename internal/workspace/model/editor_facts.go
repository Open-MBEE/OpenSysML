package model

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// CallSignature is one declaration a call may invoke: the parameters its
// argument list binds, in signature order, and the type of its result.
type CallSignature struct {
	Callee *symbols.Symbol
	Params []semantics.SignatureParameter
	// Result is the declared type of the result parameter, nil when it has none.
	Result *symbols.Symbol
	// Selected marks the overload the arguments select, as opposed to one of
	// several the call may still be read as.
	Selected bool
}

// CallSignaturesInDoc lists the declarations the call whose name ref is may
// invoke: the overload its arguments select, else the overloads they leave
// tied, else whatever the name alone resolves to. Nil for a reference that
// names no call.
func (w *Workspace) CallSignaturesInDoc(name string, ref resolve.Reference) []CallSignature {
	if ref.Invocation == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []CallSignature
	w.queryLocked(name, func(resolver *resolve.Resolver, sem *semantics.Model) {
		var callees []*symbols.Symbol
		selected := false
		if sel := invocationSelection(resolver, sem, ref); sel != nil {
			switch {
			case sel.Selected != nil:
				callees, selected = []*symbols.Symbol{sel.Selected}, true
			case len(sel.Tied) > 0:
				callees = sel.Tied
			case len(sel.Candidates) > 0:
				callees = sel.Candidates
			}
		}
		if len(callees) == 0 {
			if target, ok := resolver.ResolveReference(ref); ok && target != nil {
				callees = []*symbols.Symbol{target}
			}
		}
		for _, callee := range callees {
			out = append(out, CallSignature{
				Callee:   callee,
				Params:   sem.SignatureParametersOf(callee),
				Result:   resultTypeOf(sem, callee),
				Selected: selected,
			})
		}
	})
	return out
}

// resultTypeOf is the declared type of callee's result parameter, nil when
// callee has none or it is typed Anything.
func resultTypeOf(sem *semantics.Model, callee *symbols.Symbol) *symbols.Symbol {
	for _, p := range sem.BehaviorParametersOf(callee) {
		if !p.IsResult {
			continue
		}
		for _, t := range sem.DeclaredFeatureTypes(p.Symbol) {
			if !semantics.IsAnything(t) {
				return t
			}
		}
		return nil
	}
	return nil
}

// FeatureHint is what an editor may show beside a feature declaration: the type
// a feature declared without one is inferred to have, and the constant its
// value evaluates to from the model alone. Either may be absent.
type FeatureHint struct {
	Symbol *symbols.Symbol
	// Type is the inferred type: that of the value, else the type the features
	// it redefines or subsets give it. Nil for a feature that declares its type.
	Type *symbols.Symbol
	// Value is the constant the feature's value evaluates to, when HasValue.
	Value    semantics.Value
	HasValue bool
}

// FeatureHintsInDoc computes the hints for syms, usages declared in the named
// document, in one pass over the workspace's semantics. A value that is a
// literal gets no value hint: it already states what it is.
func (w *Workspace) FeatureHintsInDoc(name string, syms []*symbols.Symbol) []FeatureHint {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]FeatureHint, 0, len(syms))
	w.queryLocked(name, func(_ *resolve.Resolver, sem *semantics.Model) {
		for _, sym := range syms {
			usage, ok := sym.Decl.(*ast.Usage)
			if !ok {
				continue
			}
			hint := FeatureHint{Symbol: sym}
			scope := sym.OwnerScope
			if !declaresType(usage) {
				hint.Type = inferredType(sem, scope, sym, usage)
			}
			if usage.Value != nil && !isLiteral(usage.Value) && sem.ModelLevelEvaluable(scope, usage.Value) {
				hint.Value, hint.HasValue = sem.EvalIn(scope, usage.Value)
			}
			if hint.Type != nil || hint.HasValue {
				out = append(out, hint)
			}
		}
	})
	return out
}

// inferredType is the type a usage written without one is read as having.
func inferredType(sem *semantics.Model, scope *symbols.Scope, sym *symbols.Symbol, usage *ast.Usage) *symbols.Symbol {
	var typ *symbols.Symbol
	if usage.Value != nil {
		typ = sem.ExprResultType(scope, usage.Value)
	}
	if typ == nil {
		for _, t := range sem.DeclaredFeatureTypes(sym) {
			typ = t
			break
		}
	}
	if typ == nil || semantics.IsAnything(typ) || typ == sym {
		return nil
	}
	return typ
}

// declaresType reports whether the usage writes a typing of its own.
func declaresType(usage *ast.Usage) bool {
	for _, rel := range usage.Relationships {
		if rel != nil && rel.Kind == ast.RelTyping {
			return true
		}
	}
	return false
}

// isLiteral reports whether the expression is a bare literal.
func isLiteral(expr ast.Node) bool {
	switch expr.(type) {
	case *ast.LiteralBool, *ast.LiteralInteger, *ast.LiteralReal, *ast.LiteralString, *ast.LiteralInfinity, *ast.NullExpr:
		return true
	}
	return false
}
