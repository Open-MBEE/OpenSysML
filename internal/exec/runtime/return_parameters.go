package runtime

import (
	"errors"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// verdictLiteralName is the qualified name of the library's VerdictKind enumeration
// literal a verification case's verdict is read as.
const verdictLiteralName = verdictKindName + "::"

// resultParameter reports whether sym is a result parameter (`return …`) of the
// behavior declaring it, such as Cases::Case::result or RequirementCheck::result.
func resultParameter(sym *symbols.Symbol) bool {
	if sym == nil || !semantics.IsParameter(sym) {
		return false
	}
	if sym.Recorded() {
		return sym.Facts.Modifiers.Has(symbols.ModResult)
	}
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && usage.IsResult
}

// returnParameterRead answers a read of a result parameter the object's type
// inherits from the behavior it enacts: the object's own evaluation, as the library
// declares it (Requirements.sysml `RequirementCheck::result`, Cases.sysml
// `Case::result`, VerificationCases.sysml `VerificationCase::verdict`), and
// undetermined where the inputs of that evaluation are unbound.
func (ctx *Context) returnParameterRead(inst *Instance, sym *symbols.Symbol, feat *EffectiveFeature) (*FeatureValue, error) {
	val, err := ctx.returnParameterValue(inst, sym, feat)
	if err != nil {
		return nil, err
	}
	fv := &FeatureValue{Feature: feat, Materialized: true}
	if feat.Scalar() || val.Kind == ValUndetermined {
		fv.Value = val
	} else {
		fv.Values = val
	}
	return fv, nil
}

// returnParameterValue is the value of the inherited result parameter sym on inst,
// in the order the rule tries: a result parameter the object's type declares
// redefines the inherited one (KerML 8.4.4.6, implied parameter redefinition) and
// so answers for it; else the evaluation the type states — a verification case's
// verdict, a requirement's conditions; else the parameter is unbound.
func (ctx *Context) returnParameterValue(inst *Instance, sym *symbols.Symbol, feat *EffectiveFeature) (Value, error) {
	typ := inst.Type
	if own := ctx.ownResultParameter(typ, sym); own != "" {
		fv, err := inst.MemberValue(ctx, own)
		if err != nil {
			return Value{}, err
		}
		return fv.HeldValue(), nil
	}
	switch {
	case IsVerificationCaseSymbol(typ):
		return ctx.verificationVerdictValue(typ, inst, sym, feat)
	case statesConditions(typ):
		return ctx.conditionResultValue(typ, inst, sym, feat)
	}
	return unsetFeatureValue(sym.Name, feat.Multiplicity, sym), nil
}

// statesConditions reports whether sym is a definition or usage whose result is
// the conjunction of the conditions it states: a requirement, viewpoint, concern
// or a case's objective (SysML v2 §7.21, §7.22.2, Requirements.sysml `RequirementCheck::result`).
func statesConditions(sym *symbols.Symbol) bool {
	switch decl := sym.Decl.(type) {
	case *ast.Definition:
		return decl.Kind == ast.DefRequirement || decl.Kind == ast.DefViewpoint || decl.Kind == ast.DefConcern
	case *ast.Usage:
		return decl.Kind == ast.UsageRequirement || decl.Kind == ast.UsageViewpoint || decl.Kind == ast.UsageConcern ||
			decl.Kind == ast.UsageObjective || (decl.Kind == ast.UsageSatisfy && !decl.IsVerifiedRequirement())
	}
	return false
}

// ownResultParameter is the name of the result parameter a type more specific than
// sym's owner declares — the one that redefines sym (KerML 8.4.4.6, implied
// parameter redefinition) — or "" when typ inherits no more specific one.
func (ctx *Context) ownResultParameter(typ, sym *symbols.Symbol) string {
	for _, m := range ctx.model.semantics.MembersOf(typ) {
		if m == sym || !resultParameter(m) || m.Owner() == nil || m.Owner() == sym.Owner() {
			continue
		}
		if ctx.isOrSpecializes(m.Owner(), sym.Owner()) {
			return m.Name
		}
	}
	return ""
}

// conditionSelf is the object a requirement usage's conditions read: the object owner
// holds under the usage's name when it holds one (its own features, KerML §7.4.11), else owner.
func (ctx *Context) conditionSelf(sym *symbols.Symbol, owner *Instance) *Instance {
	if owner == nil || sym == nil || sym.Name == "" || declaringType(sym) == nil {
		return owner
	}
	if _, has := owner.FeatureValues[sym.Name]; !has {
		return owner
	}
	// Reaching the held object is structure a shared verdict's shape fixes, not a
	// value it reads: the fetch is kept out of the check's trace.
	tracing := ctx.tracing
	ctx.tracing = nil
	defer func() { ctx.tracing = tracing }()
	fv, err := owner.GetFeatureValue(ctx, sym.Name)
	if err != nil || fv == nil {
		return owner
	}
	value := fv.HeldValue()
	if inst, ok := ctx.instances[value.Instance]; ok && value.Kind == ValInstance {
		return inst
	}
	return owner
}

// conditionResultValue evaluates the result the library declares for a requirement
// check, `allTrue(assumptions()) implies allTrue(constraints())`, against inst's
// own features, each side as a satisfaction assertion evaluates its conditions:
// `true` when no condition is stated, undetermined when a side that decides the
// result reads an unbound input, negated for an element asserting the contrary.
func (ctx *Context) conditionResultValue(typ *symbols.Symbol, inst *Instance, sym *symbols.Symbol, feat *EffectiveFeature) (Value, error) {
	defer ctx.beginRun()()
	members := ctx.chainMembers(typ, typ.OwnerScope)
	// A nested usage's bindings name the owner's features (`subject s :>> s = sub;`) and are
	// read against it, as the check on that object reads them; its conditions read its own object.
	bindingSelf := inst
	if owner, _ := inst.Owner(); owner != nil {
		bindingSelf = owner
	}
	bindings, err := ctx.memberBindings(typ, "requirement", typ.Name, members, bindingSelf, nil, frame{})
	if err != nil {
		return ctx.conditionResultOf(err, sym, feat)
	}
	conds := ctx.conditionsOf(typ, members)
	check := conditionCheck{sym: typ, kind: "requirement", what: "require condition", self: inst, bindings: mapFrame(bindings)}
	if conflictingResultExpression(conds) != nil {
		_, err := ctx.evaluateConditions(check, conds)
		return Value{}, err
	}
	var assumed, required []Condition
	for _, cond := range conds {
		if cond.Required {
			required = append(required, cond)
		} else {
			assumed = append(assumed, requiredCondition(cond))
		}
	}
	assumptions, err := ctx.conditionsHold(check, assumed, sym, feat)
	if err != nil {
		return Value{}, err
	}
	result := impliesValue(assumptions, boolValue(true))
	if assumptions.Kind != ValConst || assumptions.Const.Bool {
		constraints, err := ctx.conditionsHold(check, required, sym, feat)
		if err != nil {
			return Value{}, err
		}
		result = impliesValue(assumptions, constraints)
	}
	// A negated satisfy asserts the requirement's result false (Constraints.sysml
	// `negatedConstraintChecks :> falseEvaluations`); the result read stays the requirement's.
	if NegatedDecl(typ) && !isSatisfyUsage(typ) && result.Kind == ValConst {
		result = boolValue(!result.Const.Bool)
	}
	return result, nil
}

// isSatisfyUsage reports a `satisfy` usage, a `verify` one included.
func isSatisfyUsage(sym *symbols.Symbol) bool {
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && usage.Kind == ast.UsageSatisfy
}

// requiredCondition is cond evaluated for its own truth, the body's nested
// conditions included, as the assumptions side of the implication needs it.
func requiredCondition(cond Condition) Condition {
	cond.Required = true
	if len(cond.Group) > 0 {
		group := make([]Condition, len(cond.Group))
		for i, nested := range cond.Group {
			group[i] = requiredCondition(nested)
		}
		cond.Group = group
	}
	return cond
}

// conditionsHold is `allTrue` over one side of the implication, three-valued:
// `true` for no condition, `false` once any condition is violated whatever the
// others read, else undetermined where one reads an unbound input.
func (ctx *Context) conditionsHold(check conditionCheck, conds []Condition, sym *symbols.Symbol, feat *EffectiveFeature) (Value, error) {
	var undetermined *Value
	for _, cond := range conds {
		holds, err := ctx.evaluateConditions(check, []Condition{cond})
		if err != nil {
			value, err := ctx.conditionResultOf(err, sym, feat)
			if err != nil {
				return Value{}, err
			}
			if value.Kind != ValConst {
				undetermined = &value
				continue
			}
			holds = value.Const.Bool
		}
		if !holds {
			return boolValue(false), nil
		}
	}
	if undetermined != nil {
		return *undetermined, nil
	}
	return boolValue(true), nil
}

// conditionResultOf maps a condition evaluation's failure: a violation is `false`,
// an unbound input is undetermined with the reason, anything else is the error.
func (ctx *Context) conditionResultOf(err error, sym *symbols.Symbol, feat *EffectiveFeature) (Value, error) {
	var violation *ViolationError
	switch {
	case errors.As(err, &violation):
		return boolValue(false), nil
	case unboundInput(err):
		return undeterminedFeatureValue(err.Error(), feat.Multiplicity, sym), nil
	}
	return Value{}, err
}

// impliesValue is `assumptions implies constraints` over values either side may
// leave undetermined: a false assumption or a true constraint fixes it.
func impliesValue(assumptions, constraints Value) Value {
	if assumptions.Kind == ValConst && !assumptions.Const.Bool {
		return boolValue(true)
	}
	if constraints.Kind == ValConst && constraints.Const.Bool {
		return boolValue(true)
	}
	if assumptions.Kind == ValConst && constraints.Kind == ValConst {
		return boolValue(false)
	}
	if assumptions.Kind != ValConst {
		return assumptions
	}
	return constraints
}

// unboundInput reports an evaluation that read an input nothing binds: an unbound
// subject, an uninitialized feature, or a condition whose value is undetermined.
func unboundInput(err error) bool {
	return errors.Is(err, errUnboundSubject) || errors.Is(err, errUninitializedFeatureValue) ||
		errors.Is(err, ErrNoValue) || errors.Is(err, ErrNeedsObject)
}

// verificationVerdictValue runs the verification case typ on inst and answers the
// VerdictKind literal its body produced, `VerdictKind::error` included; a run that
// decided nothing — an unbound subject, a body binding no verdict — is undetermined with its reason.
func (ctx *Context) verificationVerdictValue(typ *symbols.Symbol, inst *Instance, sym *symbols.Symbol, feat *EffectiveFeature) (Value, error) {
	result, err := ctx.RunVerification(typ, AnalysisArgs{}, typ.OwnerScope, inst)
	if err != nil {
		if unboundInput(err) {
			return undeterminedFeatureValue(err.Error(), feat.Multiplicity, sym), nil
		}
		return Value{}, err
	}
	verdict := result.Verdict
	if verdict.Detail != "" {
		return undeterminedFeatureValue(verdict.Detail, feat.Multiplicity, sym), nil
	}
	for _, lit := range ctx.model.resolver.Index().LookupQualified(verdictLiteralName + string(verdict.Kind)) {
		return NewEnumLiteral(lit), nil
	}
	return undeterminedFeatureValue("the library declares no VerdictKind::"+string(verdict.Kind), feat.Multiplicity, sym), nil
}

// constraintUsageRead answers a read of a constraint usage declared on inst's type —
// `constraint c { … }` or `assert constraint c { … }` — as the constraint evaluated on
// inst, the predicate the object enacts; not ok for a usage stating no body of its
// own (a typed or `::>` reference denotes its object), one bound to a value, a
// variation, or a library collection of constraint objects.
func (ec *EvalContext) constraintUsageRead(inst *Instance, name string) (Value, bool, error) {
	fv, ok := inst.FeatureValues[name]
	if !ok || fv.Feature == nil || fv.Feature.Symbol == nil || fv.Feature.Symbol.Kind != symbols.SymbolConstraintUsage {
		return Value{}, false, nil
	}
	sym := fv.Feature.Symbol
	if !semantics.StatesOwnCondition(sym) || !fv.Feature.Scalar() || fv.Feature.DefaultValue != nil ||
		ec.ctx.libraryDeclared(sym) || ec.ctx.model.semantics.IsVariationFeature(sym) || fv.Materialized || fv.HeldValue().Kind != ValInvalid {
		return Value{}, false, nil
	}
	result, err := ec.ctx.CheckConstraintOn(sym, sym.OwnerScope, inst)
	if err != nil {
		if unboundInput(err) {
			return undeterminedFeatureValue(err.Error(), fv.Feature.Multiplicity, sym), true, nil
		}
		var violation *ViolationError
		if errors.As(err, &violation) {
			return boolValue(false), true, nil
		}
		return Value{}, true, err
	}
	return boolValue(result.Holds), true, nil
}

// constraintResultTail drops a trailing `result` segment of a chain through a
// constraint usage: the constraint's value is its result parameter's.
func constraintResultTail(rest []ast.NameSegment) []ast.NameSegment {
	if len(rest) == 1 && rest[0].Text == "result" {
		return nil
	}
	return rest
}
