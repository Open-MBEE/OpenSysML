package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// evalRef evaluates a name read from its lowered record. What lowering resolved
// the name to answers in the scope it was lowered in; any other resolves the spelling.
func (ec *EvalContext) evalRef(x lowered) (Value, error) {
	if ref, ok := x.tree().(*ast.FeatureReference); ok && ref == nil {
		return Value{}, fmt.Errorf("empty feature reference")
	}
	return ec.evalName(subjectName(x.tree()), ec.resolvedHere(x))
}

// evalName evaluates a name as a reference to what it names, which is what an
// expression written as a bare name is: `rate`, `A::B::x`.
func (ec *EvalContext) evalName(qn *ast.QualifiedName, at *program.Expr) (Value, error) {
	// unset is the feature the name resolves to when a frame holds it without a
	// value: no other feature of the name may answer for it.
	var unset *symbols.Symbol
	if qn != nil && len(qn.Parts) == 1 {
		// The feature the name resolves to answers first, from the frame holding it.
		val, ok, held, err := ec.lookupHeld(qn, at)
		if err != nil {
			return Value{}, err
		} else if ok {
			return val, nil
		}
		unset = held
	}
	// Outside an expression body no body-local declaration can shadow a bound
	// name, so a frame binding is the answer: the common case, kept small.
	if qn != nil && len(qn.Parts) == 1 && (ec.scope == nil || !ec.scope.BodyLocal()) {
		if val, ok, err := ec.lookupNamed(qn.Parts[0].Text, unset); err != nil {
			return Value{}, err
		} else if ok {
			return val, nil
		}
	}
	return ec.evalNameGeneral(qn, unset, at)
}

// evalNameGeneral evaluates a name through every source that may answer it, in
// shadowing order; no other feature of the name answers for unset, a held feature with no value.
func (ec *EvalContext) evalNameGeneral(qn *ast.QualifiedName, unset *symbols.Symbol, at *program.Expr) (Value, error) {
	if qn == nil || len(qn.Parts) == 0 {
		return Value{}, fmt.Errorf("empty feature reference")
	}

	if len(qn.Parts) == 1 {
		return ec.evalSimpleName(qn, unset, at)
	}
	return ec.evalQualifiedName(qn, at)
}

// settled marks a value and error as the answer of a source that settles the name.
func settled(val Value, err error) (Value, bool, error) { return val, true, err }

// evalSimpleName evaluates a single-part name: a lookup in the frame stack, the bound instance or the scope.
func (ec *EvalContext) evalSimpleName(qn *ast.QualifiedName, unset *symbols.Symbol, at *program.Expr) (Value, error) {
	name := qn.Parts[0].Text
	// A declaration inside an expression body is local to that body and
	// shadows features of the element carrying the expression.
	if val, ok, err := ec.bodyLocalValue(name); ok {
		return val, err
	}
	// Try frame stack first (local bindings from calc/lambda params)
	if val, ok, err := ec.lookupNamed(name, unset); err != nil {
		return Value{}, err
	} else if ok {
		return val, nil
	}
	// Then a node of an action performance in the frame stack, read as a value.
	if perf, declared, err := ec.lookupSubaction(name); declared {
		if err != nil {
			return Value{}, err
		}
		return perf.resultValue()
	}
	// Then another output of the calc whose output is being computed: an `out` binding
	// may read the calc's other outputs, evaluated from the same run of its body.
	if value, ok, err := ec.calcRun.lookupOutput(ec.ctx, name); ok {
		return value, err
	}
	// Then a valued feature of the element being evaluated, masking the carrying object's member
	// and a redefined default; one whose own value is being evaluated is skipped (`in mass = mass`).
	if val, ok, err := ec.valuedFeatureValue(name); ok {
		return val, err
	}
	// Then a declared input the condition binds nothing to, which is open rather
	// than the object's same-named feature.
	if val, ok := ec.openConditionInput(name); ok {
		return val, nil
	}
	// Then the performance being run: a feature of the behavior performed is read
	// from the performance's occurrence, which masks the performer's of that name.
	if val, ok, err := ec.performanceFeatureValue(name, unset); ok {
		return val, err
	}
	// Then the bound instance and the names that stand for it: its feature, `that`, `self`, `this`.
	if val, ok, err := ec.boundInstanceName(name); ok || err != nil {
		return val, err
	}
	// Then the scope the expression was written in, as a written reference finds it; a
	// declaration's own value is evaluated in its declaring scope, whose imports answer its names.
	if val, ok, err := ec.scopeReferenceValue(name, qn, at); ok {
		return val, err
	}
	// Nothing outside the feature supplies its value, so its own value depends
	// on itself.
	if ec.resolving[name] {
		return Value{}, fmt.Errorf("%w: %s", ErrCyclicFeatureValue, name)
	}
	// A feature the element declares but nothing gives a value to is
	// uninitialized rather than unresolved.
	if _, declared := ec.features[name]; declared {
		return Value{}, &NoValueError{Feature: name}
	}
	if ec.ctx.model.resolver != nil && ec.scope != nil {
		return Value{}, fmt.Errorf("%w: %s", ErrUnresolvedReference, ec.ctx.model.resolver.UnresolvedName(ec.scope, name, qn))
	}
	return Value{}, fmt.Errorf("%w: %s", ErrUnresolvedReference, name)
}

// bodyLocalValue is the value of a declaration inside the expression body being evaluated, which is
// local to that body. ok is false when name is no such declaration.
func (ec *EvalContext) bodyLocalValue(name string) (val Value, ok bool, err error) {
	if ec.scope == nil {
		return Value{}, false, nil
	}
	sym, ok := symbols.LookupBodyLocal(ec.scope, name)
	if !ok {
		return Value{}, false, nil
	}
	// Body parameters and statement locals are supplied by the frame lookup below.
	_, bodyMember := sym.OwnerScope.Node().(*ast.BodyExpr)
	_, param := sym.Decl.(*ast.BodyExpr)
	if !bodyMember || param {
		return Value{}, false, nil
	}
	if ec.resolving[name] {
		return Value{}, true, fmt.Errorf("%w: %s", ErrCyclicFeatureValue, name)
	}
	if value := ec.ctx.extractDefaultValue(sym); value != nil {
		if ec.resolving == nil {
			ec.resolving = map[string]bool{}
		}
		ec.resolving[name] = true
		val, err := ec.declaredValue(sym, value)
		delete(ec.resolving, name)
		return val, true, err
	}
	return Value{}, true, &NoValueError{Feature: name}
}

// boundInstanceName reads name from the bound instance: a feature value it carries, `that`, `self`,
// `this`, or a Kernel Semantic Library member. ok is false when the instance does not answer it.
func (ec *EvalContext) boundInstanceName(name string) (Value, bool, error) {
	// Then the bound instance: a feature value holds the value this object actually
	// carries, which overrides the declared default the scope would yield.
	if ec.self != nil && ec.selfFeatureInScope(name) {
		// The name reads the feature it resolves to where the expression was
		// written, which the object may hold under a redefiner's name.
		if val, ok, err := ec.selfFeatureValue(ec.selfMemberSlot(name)); err != nil {
			return Value{}, true, err
		} else if ok {
			return val, true, nil
		}
	}
	// Then `that`, which every usage takes from the base usage: it names the
	// instance featuring the value being evaluated, which is the bound one.
	if name == thatName && ec.self != nil {
		return Value{Kind: ValInstance, Instance: ec.self.ID}, true, nil
	}
	// Then `self`, the thing being evaluated, read as its value.
	if ec.self != nil && ec.namesSelf(name) {
		return settled(ec.ctx.objectValue(ec.self))
	}
	// Then `this`, the context occurrence of what is being evaluated: the bound instance
	// owning the performance, named so even where no library declares it, as `that` is.
	if name == thisName && (ec.namesOccurrenceThis(name) || ec.self != nil && !ec.scopeNames(name)) {
		return settled(ec.thisValue())
	}
	// Then a Kernel Semantic Library member of the bound instance, which the
	// shape leaves out: read bare in the body as `self.<name>` reads it.
	if ec.self != nil && ec.selfFeatureInScope(name) {
		if val, answered, err := ec.kernelMemberValue(ec.self, name, ""); answered && err == nil {
			return val, true, nil
		}
	}
	return Value{}, false, nil
}

// scopeReferenceValue evaluates the declaration name resolves to in the scope the expression was
// written in. ok is false when the scope resolves nothing, or name's own value is being evaluated.
func (ec *EvalContext) scopeReferenceValue(name string, qn *ast.QualifiedName, at *program.Expr) (Value, bool, error) {
	if ec.scope == nil || ec.resolving[name] {
		return Value{}, false, nil
	}
	sym, ok := ec.lookupRef(at)
	if !ok || sym == nil {
		return Value{}, false, nil
	}
	// An inherited expression reads the feature as the running behavior
	// inherits it: through the redefinition, when it states one.
	sym = ec.ctx.inheritedFeature(ec.runningBehavior(), sym)
	ec.ctx.noteDeclarationRead(sym)
	// An enumerated value is the value of its enumeration it stands
	// for; any other variant names a choice, not the value it declares.
	if semantics.EnumerationOwning(sym) != nil {
		return settled(ec.enumLiteralValue(sym))
	}
	if ec.ctx.model.semantics.VariationPointOwning(sym) != nil {
		return variantReference(sym), true, nil
	}
	// A feature of an enclosing type, named from a nested usage, is read from the object
	// enclosing the bound one: `e1` inside `e3` is the containing rectangle's e1.
	if val, ok, err := ec.outerFeatureValue(sym); ok {
		return val, true, err
	}
	// A transformation's target is the frame featuring it, which its
	// value carries and no object states.
	if val, ok, err := ec.featuringReferenceValue(sym); ok {
		return val, true, err
	}
	// A library feature's value comes from the feature seam, not its
	// declared body: a warm library cache restores symbols without AST.
	if val, ok, err := ec.ctx.libraryFeatureValue(sym); ok {
		return val, true, err
	}
	// A measurement unit declaration is the measurement reference it names.
	if val, ok, err := ec.ctx.MeasurementUnitValue(sym); ok {
		return val, true, err
	}
	// A connection a behavior being performed declares is that performance's own connector.
	if val, ok, err := ec.performedConnector(sym); ok {
		return val, true, err
	}
	// A part, item or structured value names an object, so the name evaluates to
	// that object rather than to a value the declaration would have to hold.
	if val, ok, err := ec.occurrenceReference(sym); ok {
		return val, true, err
	}
	// A calc definition, or a calc usage awaiting arguments, is a function.
	if val, ok, err := ec.calcAsValue(sym); ok {
		return val, true, err
	}
	// A calc usage returning one unnamed result is read as that result.
	if lower.IsCalcUsageSymbol(sym) && ec.ctx.returnsResult(sym) {
		return settled(ec.evalCalcUsageMembers(sym, resultSegments))
	}
	// A feature declared with no value, whose multiplicity admits none,
	// states the empty sequence — what an object holding nothing reads.
	if val, ok := ec.emptyDeclaredFeature(sym); ok {
		return val, true, nil
	}
	if value := ec.ctx.extractDefaultValue(sym); value != nil {
		if ec.resolving == nil {
			ec.resolving = map[string]bool{}
		}
		ec.resolving[name] = true
		val, err := ec.declaredValue(sym, value)
		delete(ec.resolving, name)
		return val, true, err
	}
	// A variation holds nothing until it is bound, whether it is read
	// through an object or through its declaration.
	if ec.ctx.model.semantics.IsVariationFeature(sym) {
		return Value{}, true, fmt.Errorf("%w: %s", ErrVariationUnselected, name)
	}
	return settled(ec.resolvedWithoutValue(sym, qn))
}

// evalQualifiedName evaluates a multi-part name through the symbol the checker resolved it to.
func (ec *EvalContext) evalQualifiedName(qn *ast.QualifiedName, at *program.Expr) (Value, error) {
	// A multi-part name A::B::x resolves as the checker resolves it, so the two agree, and
	// in this evaluation's scope, since one expression may be evaluated in several.
	currentSym := at.Ref.Sym
	ec.ctx.noteQualifiedRead(ec.scope, qn, currentSym, currentSym != nil)

	// A path starting at a node of an action performance on the stack reads
	// through that node's performance: `p.v`, `leg.inner.v`.
	if perf, declared, err := ec.lookupSubaction(qn.Parts[0].Text); declared && !qn.Global {
		if err != nil {
			return Value{}, err
		}
		return ec.evalSubactionPath(perf, qn.Parts[1:])
	}

	// A calc usage's output features are computed rather than declared values,
	// so a name qualified by one reads the rest from an evaluation of the usage.
	if usage, rest, ok := calcUsageQualifier(qn, at); ok {
		return ec.evalCalcUsageMembers(usage, rest)
	}
	if currentSym == nil {
		// The resolver's own reading says why the name denotes nothing.
		return Value{}, ec.unresolvedQualifiedName(qn, ec.ctx.model.resolver.ReadQualified(ec.scope, qn))
	}
	ec.ctx.noteDeclarationRead(currentSym)

	// A feature of a behavior whose run is on the stack (`MassCase::result` in its
	// objective or assertion) reads the value that run bound to it.
	if qualifier, ok := partOf(at, len(qn.Parts)-2); ok {
		if val, ok, err := ec.frameFeatureValue(qualifier, currentSym); err != nil {
			return Value{}, err
		} else if ok {
			return val, nil
		}
	}
	// A satisfy usage's `by` subject awaits its satisfaction's binding, as a subject does.
	if rel, ok := currentSym.Decl.(*ast.Relationship); ok && rel.Kind == ast.RelSubject {
		return ec.resolvedWithoutValue(currentSym, qn)
	}
	// A library feature reads through the feature seam, whatever the library
	// declares for it and whether or not the cache kept its declaration.
	if val, ok, err := ec.ctx.libraryFeatureValue(currentSym); ok {
		return val, err
	}
	// A measurement unit declaration (`SI::m`) is the measurement reference it names.
	if val, ok, err := ec.ctx.MeasurementUnitValue(currentSym); ok {
		return val, err
	}
	// A qualified feature of the behavior performed (`SM::entryAction`) is the performance's.
	if val, ok, err := ec.performanceFeatureOf(currentSym); ok {
		return val, err
	}
	// A qualified feature of an enclosing type (`Rectangle::length` inside its
	// `e1`) reads the enclosing object's value of that feature.
	if val, ok, err := ec.outerFeatureValue(currentSym); ok {
		return val, err
	}

	// A calc definition, or a calc usage awaiting arguments, is a function.
	if val, ok, err := ec.calcAsValue(currentSym); ok {
		return val, err
	}

	// Evaluate the final symbol's declaration
	if decl, ok := currentSym.Decl.(*ast.Usage); ok {
		if val, ok, err := ec.usageValue(currentSym, decl, qn); ok {
			return val, err
		}
	}
	// A require/assume constraint reads the value it binds, as a constraint usage does.
	if oc, ok := ast.OwnedConstraintOf(currentSym.Decl); ok && oc.Value != nil {
		return ec.declaredValue(currentSym, oc.Value)
	}
	// A subject is bound or, admitting nothing, empty; otherwise it awaits a binding.
	if decl, ok := currentSym.Decl.(*ast.SubjectMember); ok {
		if decl.BindingExpr != nil {
			return ec.declaredValue(currentSym, decl.BindingExpr)
		}
		if val, ok, err := ec.ctx.emptyOptionalDeclaration(currentSym); ok || err != nil {
			return val, err
		}
	}
	return ec.resolvedWithoutValue(currentSym, qn)
}

// calcUsageQualifier finds the first calc usage qualifying the name, with the parts that follow it.
func calcUsageQualifier(qn *ast.QualifiedName, at *program.Expr) (*symbols.Symbol, []ast.NameSegment, bool) {
	for i := 0; i < len(qn.Parts)-1; i++ {
		part, resolved := partOf(at, i)
		if !resolved {
			break
		}
		if lower.IsCalcUsageSymbol(part) {
			return part, qn.Parts[i+1:], true
		}
	}
	return nil, nil, false
}

// usageValue is what a qualified name ending at a usage evaluates to. ok is false when the usage
// states nothing itself, leaving the name to the declaration forms that follow.
func (ec *EvalContext) usageValue(currentSym *symbols.Symbol, decl *ast.Usage, qn *ast.QualifiedName) (Value, bool, error) {
	// An enumerated value is a value of its enumeration; any other variant names a choice
	// its variation can be bound to, and compares equal to the variation that selected it.
	if semantics.EnumerationOwning(currentSym) != nil {
		return settled(ec.enumLiteralValue(currentSym))
	}
	if ec.ctx.model.semantics.VariationPointOwning(currentSym) != nil {
		return variantReference(currentSym), true, nil
	}
	if decl.Value != nil {
		return settled(ec.declaredValue(currentSym, decl.Value))
	}
	if ec.ctx.model.semantics.IsVariationFeature(currentSym) {
		return Value{}, true, fmt.Errorf("%w: %s", ErrVariationUnselected, qualifiedNameToString(qn))
	}
	// A part, item or structured value names an object, read as that object.
	if val, ok, err := ec.occurrenceReference(currentSym); ok {
		return val, true, err
	}
	// A calc usage returning one unnamed result is read as that result.
	if lower.IsCalcUsageSymbol(currentSym) && ec.ctx.returnsResult(currentSym) {
		return settled(ec.evalCalcUsageMembers(currentSym, resultSegments))
	}
	// A calc usage or a KerML type is never the empty sequence, whatever it admits.
	if lower.IsCalcUsageSymbol(currentSym) || declaresType(currentSym) {
		return settled(ec.resolvedWithoutValue(currentSym, qn))
	}
	// A valueless feature admitting nothing is the empty sequence, however spelled.
	if val, ok := ec.emptyDeclaredFeature(currentSym); ok {
		return val, true, nil
	}
	return Value{}, false, nil
}

// resolvedWithoutValue reads a name that resolves to sym but no value: undetermined
// at model level, uninitialized (not unresolved) when an object features it.
func (ec *EvalContext) resolvedWithoutValue(sym *symbols.Symbol, qn *ast.QualifiedName) (Value, error) {
	return ec.withoutValue(sym, qualifiedNameToString(qn))
}

// withoutValue reads sym, spelled as written, when it binds no value.
func (ec *EvalContext) withoutValue(sym *symbols.Symbol, spelled string) (Value, error) {
	// Definitions are types, not values.
	if declaresType(sym) {
		return Value{}, fmt.Errorf("cannot evaluate definition %s", spelled)
	}
	// A calc usage is an evaluation, not a value: it is read through the outputs it
	// computes, since a name it designates no result for has no one value.
	if lower.IsCalcUsageSymbol(sym) {
		return Value{}, fmt.Errorf(
			"%w: calc usage %s computes output features (%s); read one of them",
			ErrNoValue, spelled, ec.ctx.calcUsageOutputSummary(sym),
		)
	}
	// A usage of any kind — a subject or a state included — is a feature.
	if _, usage := sym.Decl.(*ast.Usage); usage || semantics.IsShapeFeature(sym) {
		if ec.modelLevel() {
			return ec.undeterminedFeature(sym, spelled), nil
		}
		return Value{}, &NoValueError{Feature: spelled}
	}
	return Value{}, fmt.Errorf("cannot evaluate %s %s", sym.Kind, spelled)
}

// unresolvedQualifiedName reports a multi-part name the resolver rejected: as ambiguous, else
// against the variants or literals of what the deepest resolved segment reached.
func (ec *EvalContext) unresolvedQualifiedName(qn *ast.QualifiedName, reading resolve.Reading) error {
	written := qualifiedNameToString(qn)
	if qn.Global {
		written = "$::" + written
	}
	if n, ok := reading.Ambiguity(); ok {
		return fmt.Errorf("%w: %s (%d candidates)", errAmbiguousReference, written, n)
	}
	for i := len(qn.Parts) - 2; i >= 0; i-- {
		owner, ok := reading.Part(i)
		if !ok {
			continue
		}
		memberName := qn.Parts[i+1].Text
		if semantics.IsEnumeration(owner) {
			return fmt.Errorf("%w: %s is not a literal of %s (%s)",
				errNotALiteral, memberName, owner.Name, ec.ctx.enumerationSummary(owner))
		}
		if ec.ctx.model.semantics.IsVariationFeature(owner) {
			return fmt.Errorf("%w: %s is not a variant of %s (%s)",
				errNotAVariant, memberName, owner.Name, ec.ctx.variantSummary(owner))
		}
		if ec.ctx.model.resolver != nil {
			return fmt.Errorf("%w: %s", ErrUnresolvedReference, ec.ctx.model.resolver.UnresolvedMember(ec.scope, qn, owner, i+1))
		}
		break
	}
	return fmt.Errorf("%w: %s", ErrUnresolvedReference, written)
}

// resolveClassificationType resolves the type a classification names, seeing
// through an alias to the type it stands for.
func (ec *EvalContext) resolveClassificationType(qn *ast.QualifiedName, at *program.Expr) (*symbols.Symbol, bool) {
	target := at.Ref.Sym
	ec.ctx.noteQualifiedRead(ec.scope, qn, target, target != nil)
	if target == nil {
		return nil, false
	}
	if canonical, ok := ec.ctx.resolveAliasTarget(target); ok {
		target = canonical
	}
	ec.ctx.noteDeclarationRead(target)
	return target, true
}
