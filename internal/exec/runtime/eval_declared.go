package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// lookupSubaction is the latest performance of the node named name in a performance on the stack,
// innermost first: the declaration's node where name resolves, none where a feature shadows it.
func (ec *EvalContext) lookupSubaction(name string) (perf *actionFrame, declared bool, err error) {
	if !ec.hasPerformanceFrame() {
		return nil, false, nil
	}
	var decl ast.Node
	if ec.ctx.model.resolver != nil {
		if sym, ok := ec.lookupName(name); ok && sym != nil {
			if usage, ok := sym.Decl.(*ast.Usage); ok && !usage.Kind.IsAction() && !lower.IsCaseNode(usage) {
				return nil, false, nil
			}
			decl = sym.Decl
		}
	}
	for i := len(ec.frames) - 1; i >= 0; i-- {
		f := ec.frames[i].perf
		if f == nil {
			continue
		}
		if perf, declared, err = f.subaction(name, decl); declared {
			return perf, true, err
		}
	}
	return nil, false, nil
}

// evalSubactionPath reads `node.pin` or `node.inner.pin` through the named nodes' performances;
// the path past a pin chains through its value, and one ending at a node reads its result.
func (ec *EvalContext) evalSubactionPath(perf *actionFrame, parts []ast.NameSegment) (Value, error) {
	for i, part := range parts {
		if inner, declared, err := perf.subaction(part.Text, nil); declared {
			if err != nil {
				return Value{}, err
			}
			perf = inner
			continue
		}
		if i != len(parts)-1 && !perf.declares(part.Text) {
			return Value{}, fmt.Errorf("%w: %s declares no node or pin %s to read %s through",
				errNodePin, perf.describe(), part.Text, parts[len(parts)-1].Text)
		}
		var value Value
		var err error
		if ec.readsAcross(perf) {
			value, err = perf.repeatedPin(part.Text)
		} else {
			value, err = perf.pin(part.Text)
		}
		if err != nil {
			return Value{}, err
		}
		return ec.chainMemberValue(value, parts[i+1:], perf.path()+"."+part.Text)
	}
	if ec.readsAcross(perf) && perf.result != "" {
		return perf.repeatedPin(perf.result)
	}
	return perf.resultValue()
}

// declaredElements types a read yielding no element by the quantity dimension it declares
// (KerML 8.4.4.9), in that dimension's coherent unit, so an aggregate of it keeps the kind.
func (ec *EvalContext) declaredElements(node ast.Node, val Value, err error) (Value, error) {
	if err != nil || !isEmptyValue(val) {
		return val, err
	}
	if typed, ok := ec.ctx.emptyOfDeclared(ec.scope, node); ok {
		return typed, nil
	}
	return val, nil
}

// emptyOfDeclared is the empty sequence of the quantities an expression is declared to yield, in
// their coherent unit; false where the declarations fix no dimension or a dimensionless one.
func (ctx *Context) emptyOfDeclared(scope *symbols.Scope, node ast.Node) (Value, bool) {
	if ctx.model.semantics == nil || scope == nil {
		return Value{}, false
	}
	return ctx.emptyOfDimension(ctx.model.semantics.DimensionOfExpr(scope, node))
}

// declaredCount is how many values an expression holds wherever it is evaluated,
// as far as the declarations it reads fix that: `[0..*]` where they leave it open.
func (ec *EvalContext) declaredCount(scope *symbols.Scope, node ast.Node) semantics.Range {
	if ec.ctx.model.semantics == nil || scope == nil {
		return openRange()
	}
	if count, ok := ec.ctx.model.semantics.ValuesHeldBy(scope, node); ok {
		return count
	}
	return openRange()
}

// declaredValue evaluates a declaration's bound value in its declaring scope, answering to the
// declared type; a namespace-level object usage's is one binding (KerML 1.0 §7.4.11), kept for the run.
func (ec *EvalContext) declaredValue(sym *symbols.Symbol, value ast.Node) (Value, error) {
	ec.ctx.noteDeclarationRead(sym)
	if val, ok := ec.ctx.namespaceBindings[sym]; ok {
		return val, nil
	}
	if !namespaceObjectUsage(sym) {
		leave, err := ec.ctx.enterBinding(sym)
		if err != nil {
			return Value{}, err
		}
		defer leave()
		return ec.evaluateDeclared(sym, value)
	}
	// A binding connector joining this usage makes its ends denote the same
	// values; resolving it records the binding before the value is read.
	if _, bound, err := ec.ctx.namespaceBoundObjects(sym); err != nil {
		return Value{}, err
	} else if bound {
		if val, ok := ec.ctx.namespaceBindings[sym]; ok {
			return val, nil
		}
		// The class's member declaring sym may be a different scope tree's
		// symbol for it; its recorded binding is this usage's value too.
		if _, member := ec.ctx.namespaceClassMember(sym); member != sym {
			if val, ok := ec.ctx.namespaceBindings[member]; ok {
				return val, nil
			}
		}
	}
	leave, err := ec.ctx.enterBinding(sym)
	if err != nil {
		return Value{}, err
	}
	defer leave()
	// The binding is made whole or not at all: a value refused after constructing
	// objects leaves none of them, nor their behaviors, behind.
	commit, rollback := ec.ctx.beginJournal()
	val, err := ec.evaluateDeclared(sym, value)
	if err != nil {
		rollback()
		delete(ec.ctx.bindingReads, sym)
		return Value{}, err
	}
	ec.ctx.bindNamespace(sym, val)
	commit()
	return val, nil
}

// evaluateDeclared evaluates a declaration's value anew, answering to its declared type.
func (ec *EvalContext) evaluateDeclared(sym *symbols.Symbol, value ast.Node) (Value, error) {
	val, err := ec.evalIn(sym.OwnerScope).Eval(value)
	if err != nil {
		return Value{}, err
	}
	return ec.conformDeclared(sym, val)
}

// orderingGap says which Kernel Function Library function would have to declare ordering op
// for val and why none does: only the numeric libraries and StringFunctions declare it concretely.
func (ctx *Context) orderingGap(op program.Operator, val Value) string {
	fn := func(pkg string) string { return fmt.Sprintf("%s::'%s'", pkg, op) }
	notScalar := func(what string) string {
		return fmt.Sprintf("%s is abstract and no library function declares '%s' for %s, which is no ScalarValue", fn("DataFunctions"), op, what)
	}
	switch val.Kind {
	case ValComplex:
		return fmt.Sprintf("%s is abstract and ComplexFunctions declares no '%s' for Complex", fn("NumericalFunctions"), op)
	case ValEnumLiteral:
		enum := semantics.EnumerationOwning(val.Literal())
		if enum == nil {
			return notScalar(val.LiteralText())
		}
		if ctx.conformsToLibrary(enum, "ScalarValues::ScalarValue") {
			return fmt.Sprintf("the library orders %s by value and %s declares none", enum.Name, val.LiteralText())
		}
		return notScalar("the enumeration " + enum.Name)
	case ValNull, ValSequence, ValSet:
		return fmt.Sprintf("%s takes one DataValue per operand", fn("DataFunctions"))
	case ValInstance, ValVariant:
		what := describeOperand(val)
		if types, err := ctx.directValueTypes(nil, val); err == nil && len(types) > 0 {
			what = fmt.Sprintf("an instance of the %s %s", ast.Notation(types[0].Decl), types[0].Name)
		}
		if ctx.isDataValue(val) {
			return notScalar(what)
		}
		return fmt.Sprintf("%s takes DataValue operands and %s is none", fn("DataFunctions"), what)
	case ValFunction, valExpr:
		return fmt.Sprintf("%s takes DataValue operands and %s is none", fn("DataFunctions"), describeOperand(val))
	}
	return notScalar(describeOperand(val))
}

// valueOperand evaluates an operand an operator needs one value of: one holding none is
// reported as such, and a one-element collection is the value it holds.
func (ec *EvalContext) valueOperand(node ast.Node) (Value, error) {
	return ec.valueOperandExpr(ec.lowered(node))
}
