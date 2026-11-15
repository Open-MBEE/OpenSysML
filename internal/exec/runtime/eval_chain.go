package runtime

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// evalFeatureChain evaluates a feature chain expression (e.g., obj.member.submember).
func (ec *EvalContext) evalFeatureChain(x lowered) (Value, error) {
	if len(x.expr().Chain) == 0 {
		return Value{}, fmt.Errorf("empty member chain")
	}
	// A member is declared by what holds it at run time, so the members are read by
	// their spelling, which the helpers shared with the other chain forms take as written.
	n, _ := x.tree().(*ast.FeatureChainExpr)
	base, parts := ast.FeatureChainBase(n)
	defer ec.restoreChain(ec.noteChainMembers(n, base, parts))

	// A chain's members resolve in the namespace of the feature before them, where
	// a redefined feature is not inherited under its name (KerML §7.3.4.5).
	if err := ec.maskedChainMember(base, parts); err != nil {
		return Value{}, err
	}

	// A state's activity, `fill.isActive`, is read from the active configuration
	// of the machine holding the state, not from a feature value.
	if val, ok, err := ec.stateActivity(n, base, parts); ok {
		return val, err
	}

	// A node of an action performance on the stack carries its pins in its own
	// performance, which `p.v` reads.
	if name := simpleEndName(base); name != "" {
		if perf, declared, err := ec.lookupSubaction(name); declared {
			if err != nil {
				return Value{}, err
			}
			return ec.evalSubactionPath(perf, parts)
		}
	}

	// A transition's payload, `T.d`, is what its trigger bound in the firing the
	// reading behavior is performed within.
	if val, ok, err := ec.transitionPayload(base, parts); ok {
		return val, err
	}

	// A calc usage carries no value of its own: `c.a` runs the usage, once, and reads
	// the output from that evaluation rather than from a feature value.
	if sym, ok, err := ec.calcUsageOperand(base); err != nil {
		return Value{}, err
	} else if ok {
		return ec.evalCalcUsageMembers(sym, parts)
	}

	// A constraint usage of the object being evaluated is the predicate it enacts,
	// read through the object (`fits.limit` as `self.fits.limit`).
	if val, ok, err := ec.enactedConstraintChain(base, parts); ok {
		return val, err
	}

	// A variant named through the variation usage it belongs to is that variant,
	// `engine.electric.power`, not a member of what the usage selects.
	if sym, ok := ec.chainBaseSymbol(base); ok {
		if val, ok, err := ec.chainThroughNamedVariant(sym, parts, ec.readingObject()); ok {
			return val, err
		}
	}

	// A part carries no value of its own: it denotes an occurrence, whose features
	// `lander.mass.mDry` reads, so the chain is read from that object.
	if sym, ok, err := ec.occurrenceOperand(base); err != nil {
		return Value{}, err
	} else if ok {
		return ec.occurrenceChainValue(sym, parts)
	}

	// One chain is one read from its first operand through every member in turn
	// (KerML 1.0 §7.4.9.3), not a read off the value of a shorter chain.
	operand, err := ec.evalExpr(x.arg(0))
	if err != nil {
		return ec.valuelessChain(err, base, parts)
	}
	return ec.operandChainValue(operand, base, parts)
}

// enactedConstraintChain reads a chain whose base names a constraint usage the
// object being evaluated enacts through that object, unless the name has a value.
func (ec *EvalContext) enactedConstraintChain(base ast.Node, parts []ast.NameSegment) (Value, bool, error) {
	name := simpleEndName(base)
	if name == "" || ec.self == nil {
		return Value{}, false, nil
	}
	if _, enacted := ec.enactedConstraint(ec.self, name); !enacted {
		return Value{}, false, nil
	}
	if names, err := ec.namesValue(name); err != nil {
		return Value{}, true, err
	} else if names {
		return Value{}, false, nil
	}
	val, err := ec.chainMemberValue(Value{Kind: ValInstance, Instance: ec.self.ID}, append([]ast.NameSegment{{Text: name}}, parts...), "")
	return val, true, err
}

// occurrenceChainValue reads the chain's members from the object sym denotes.
func (ec *EvalContext) occurrenceChainValue(sym *symbols.Symbol, parts []ast.NameSegment) (Value, error) {
	// A usage of the behavior performed is read from its performance, so `fresh.n`
	// inside `go` reads the part the performance holds, as a bare `fresh` does.
	if val, ok, err := ec.performanceFeatureOf(sym); ok {
		if err != nil {
			return Value{}, err
		}
		return ec.chainMemberValue(val, parts, sym.Name)
	}
	// A usage of an enclosing object is read from that object, so a sibling
	// chain `e1.length` inside `e3` reads the containing rectangle's e1.
	if val, ok, err := ec.outerFeatureValue(sym); ok {
		if err != nil {
			return Value{}, err
		}
		return ec.chainMemberValue(val, parts, sym.Name)
	}
	// The objects read as their values, whose own members they answer before the objects'.
	val, err := ec.ctx.denotedValue(sym)
	if err != nil {
		return Value{}, err
	}
	return ec.chainMemberValue(val, parts, sym.Name)
}

// valuelessChain answers a chain whose base has no value: unresolved when a
// member is undeclared, the declared member value when there is one, else err.
func (ec *EvalContext) valuelessChain(err error, base ast.Node, parts []ast.NameSegment) (Value, error) {
	var noValue *NoValueError
	if !errors.As(err, &noValue) {
		return Value{}, err
	}
	if unresolved := ec.chainMembersDeclared(base, parts); unresolved != nil {
		return Value{}, unresolved
	}
	// A usage no object stands for answers the chain through the
	// value its member declares, as `usage::member` reads it.
	if val, ok, err := ec.declaredChainValue(base, parts); ok {
		return val, err
	}
	return Value{}, err
}

// operandChainValue reads the chain's members from the value its base evaluated to.
func (ec *EvalContext) operandChainValue(operand Value, base ast.Node, parts []ast.NameSegment) (Value, error) {
	if operand.Kind == ValInstance {
		if _, ok := ec.ctx.instances[operand.Instance]; !ok {
			return Value{}, fmt.Errorf("instance ID %d not found", operand.Instance)
		}
	}
	// An undetermined value a feature holds is chained as that feature's, so its
	// members are the ones the feature declares however the value was computed.
	if u := operand.Undetermined(); u != nil && u.feature == nil {
		if sym, ok := ec.chainBaseSymbol(base); ok {
			operand = undeterminedFeatureValue(u.reason, u.count, sym)
		}
	}
	// At model level a valueless usage reads undetermined, but a member it
	// binds a value to is determined all the same.
	if operand.Kind == ValUndetermined {
		if val, ok, err := ec.declaredChainValue(base, parts); ok {
			return val, err
		}
	}
	return ec.chainMemberValue(operand, parts, "")
}

// chainMembersDeclared reports the first chain member nothing declares, so
// `wheels.nonexistent` is unresolved rather than unset when wheels has no value.
func (ec *EvalContext) chainMembersDeclared(base ast.Node, parts []ast.NameSegment) error {
	cur, ok := ec.chainBaseSymbol(base)
	if !ok {
		return nil
	}
	for _, part := range parts {
		next, ok := ec.ctx.declaredMember(cur, part.Text)
		if !ok {
			return fmt.Errorf("%w: %s has no member %s", ErrUnresolvedReference, cur.Name, part.Text)
		}
		cur = next
	}
	return nil
}

// chainBaseSymbol is the feature a chain's base names, when it names one, or
// the type a cast base (`x as T`) classifies its values by.
func (ec *EvalContext) chainBaseSymbol(base ast.Node) (*symbols.Symbol, bool) {
	if cast, ok := base.(*ast.OperatorExpr); ok && cast.Operator == ast.OpAs && cast.TypeRef != nil {
		return ec.resolveClassificationType(cast.TypeRef, ec.lowered(base).expr())
	}
	ref, ok := base.(*ast.FeatureReference)
	if !ok || ref.Name == nil || ec.ctx.model.resolver == nil {
		return nil, false
	}
	sym, ok := ec.ctx.resolveQualified(ec.scope, ref.Name)
	return sym, ok && sym != nil
}

// maskedChainMember is the static read's error for a segment naming a feature a redefinition
// removed from the namespace of the segment before it (KerML §7.3.4.5, §8.3.3.1.10), else nil.
func (ec *EvalContext) maskedChainMember(base ast.Node, parts []ast.NameSegment) error {
	cur, ok := ec.chainBaseSymbol(base)
	if !ok {
		return nil
	}
	for _, part := range parts {
		feat, masked := ec.ctx.shapeMember(cur, part.Text)
		if feat == nil {
			return nil
		}
		if masked {
			if redefiner := ec.ctx.redefiningMember(cur, part.Text); redefiner != nil {
				return fmt.Errorf("%w: %s has no member %s: %s redefines it", ErrUnresolvedReference, cur.Name, part.Text, ec.ctx.QualifiedName(redefiner))
			}
			return fmt.Errorf("%w: %s has no member %s: a redefinition removes it", ErrUnresolvedReference, cur.Name, part.Text)
		}
		cur = feat
	}
	return nil
}

// chainMemberValue reads the members parts name from value, through each object an intermediate
// member names, in order and flattened one level (KerML 1.0 §7.3.4.6); from names value's source.
func (ec *EvalContext) chainMemberValue(value Value, parts []ast.NameSegment, from string) (Value, error) {
	if len(parts) == 0 {
		if inst, ok := ec.ctx.instances[value.Instance]; ok && value.Kind == ValInstance {
			return ec.ctx.objectValue(inst)
		}
		return value, nil
	}
	if literal := value.EnumerationLiteral(); literal != nil {
		// A literal is an occurrence of its enumeration, so its own features are
		// read from the object that literal stands for, whatever scalar it equals.
		inst, err := ec.ctx.enumLiteralObject(literal)
		if err != nil {
			return Value{}, err
		}
		return ec.chainMemberValue(Value{Kind: ValInstance, Instance: inst.ID}, parts, from)
	}
	switch value.Kind {
	case ValSequence, ValSet:
		value, err := ec.ctx.heldInFull(value)
		if err != nil {
			return Value{}, err
		}
		return ec.chainOverElements(value, parts, from)
	case ValArray, ValVector, ValVectorQuantity, ValTensorQuantity, ValQuantity, ValMeasurementRef, ValCoordinateFrame, ValCoordinateTransformation:
		return ec.chainThroughStructured(value, parts, from)
	case ValInstance:
		// handled below
	case ValVariant:
		// A variant named with no selecting object stands for its value or an object of it.
		if value.Instance == 0 {
			stood, err := ec.variantStandsFor(value)
			if err != nil {
				return Value{}, err
			}
			if stood.Kind != ValVariant || stood.Instance != 0 {
				return ec.chainMemberValue(stood, parts, from)
			}
		}
	case ValMetaobject:
		// A metaobject answers its metaclass's features for the element it denotes.
		return ec.chainOwnFeature(value, parts, from)
	case ValUndetermined:
		return ec.chainThroughUndetermined(value, parts, from)
	default:
		if err := metadataOfAValue(value, parts); err != nil {
			return Value{}, err
		}
		return Value{}, fmt.Errorf("cannot chain through non-instance member %s (%v)", from, value.Kind)
	}

	return ec.chainObjectMember(value, parts, from)
}

// chainObjectMember continues a chain through a member of the object an instance or
// selected variant materialized: what the object is answers first, then its features.
func (ec *EvalContext) chainObjectMember(value Value, parts []ast.NameSegment, from string) (Value, error) {
	name, rest := parts[0].Text, parts[1:]
	// A selected variant is chained through the object it materialized.
	id, isObject := value.Object()
	if !isObject {
		if err := metadataOfAValue(value, parts); err != nil {
			return Value{}, err
		}
		return Value{}, fmt.Errorf("cannot chain through non-instance member %s (%v)", from, value.Kind)
	}
	inst, ok := ec.ctx.instances[id]
	if !ok {
		return Value{}, fmt.Errorf("instance ID %d not found for member %s", id, from)
	}
	// A frame, scale or transformation object answers its members from the value it is.
	if ref, isRef, err := ec.ctx.referenceValueOfObject(inst); isRef {
		if err != nil {
			return Value{}, err
		}
		return ec.chainMemberValue(ref, parts, from)
	}
	// A shaped Array object answers Array's features and their redefinitions from
	// the value; its other members stay the object's, whatever their names.
	if arr, isArray, err := ec.ctx.arrayOfObject(inst); isArray {
		if err != nil {
			return Value{}, err
		}
		if base, member, ok := ec.ctx.arrayFeatureNamed(inst.Type, name); ok {
			answer, err := ec.ctx.structuredMember(arr, base, member, inst.Type)
			if err != nil {
				return Value{}, err
			}
			return ec.chainMemberValue(answer, rest, name)
		}
	}
	// A clock's currentTime is the run's shared clock, which no feature value holds.
	if now, isClock, err := ec.ctx.clockMember(inst, name); isClock {
		if err != nil {
			return Value{}, err
		}
		return ec.chainMemberValue(now, rest, name)
	}
	// The segment reads the feature its name resolved to, which the object may
	// hold under the name of a feature redefining it (KerML 1.0 §7.3.4.5).
	if slot, ok := ec.ctx.featureSlot(inst, ec.chainMember(parts)); ok {
		name = slot
	}
	fvDecl, ok := inst.FeatureValues[name]
	if !ok {
		return ec.chainUnheldMember(inst, name, rest, from)
	}
	// A variant named through the variation feature it belongs to is the choice
	// itself, not a member of the variation's value.
	if fvDecl.Feature != nil {
		if val, ok, err := ec.chainThroughNamedVariant(fvDecl.Feature.Symbol, rest, inst.ID); ok {
			return val, err
		}
	}
	return ec.chainFeatureValue(inst, name, rest, from)
}

// chainThroughStructured continues a chain through an array, vector, quantity or frame
// value: its own features answer from the value, the rest from the object it was read from.
func (ec *EvalContext) chainThroughStructured(value Value, parts []ast.NameSegment, from string) (Value, error) {
	inst, ok := ec.ctx.structuredObject(value)
	if !ok || !(isStructuredValue(&value) || value.Kind == ValQuantity) {
		return ec.chainOwnFeature(value, parts, from)
	}
	// A scalar quantity answers its own features (`num`, `mRef`) as the value it
	// is; its object answers the members a specialization adds.
	next, segments, label := Value{Kind: ValInstance, Instance: inst.ID}, parts, from
	if value.Kind == ValQuantity {
		if answer, ok, err := ec.ctx.quantityFeature(value, parts[0].Text); ok && err == nil {
			next, segments, label = answer, parts[1:], parts[0].Text
		}
	}
	return ec.chainMemberValue(next, segments, label)
}

// chainUnheldMember continues a chain through a member no feature value holds: a calc
// usage's run, a derived Kernel collection, a holder-governed default or a Kernel member.
func (ec *EvalContext) chainUnheldMember(inst *Instance, name string, rest []ast.NameSegment, from string) (Value, error) {
	// A calc usage is an evaluation rather than a feature value, so its outputs are
	// read from a run of it against this object.
	if sym, found := ec.ctx.model.semantics.LookupMember(inst.Type, name); found && lower.IsCalcUsageSymbol(sym) {
		return ec.calcUsageMemberValue(sym, inst, rest)
	}
	if ec.ctx.derivesKernelCollection(inst, name) {
		return ec.chainFeatureValue(inst, name, rest, from)
	}
	// A member the shape leaves out, which the default the holder governs over
	// states; else a Kernel member.
	val, answered, err := ec.ctx.holderDefaultMember(inst, name)
	if !answered {
		val, answered, err = ec.kernelMemberValue(inst, name, from)
	}
	if answered {
		if err != nil {
			return Value{}, err
		}
		return ec.chainMemberValue(val, rest, name)
	}
	return Value{}, fmt.Errorf("%w: member %s not found in instance", ErrNoSuchFeature, name)
}

// chainFeatureValue continues a chain through the named feature value of inst.
func (ec *EvalContext) chainFeatureValue(inst *Instance, name string, rest []ast.NameSegment, from string) (Value, error) {
	if sym, ok := ec.enactedConstraint(inst, name); ok {
		// The constraint's own members read through it as any usage's do; the
		// usage itself, or its `result`, is the predicate evaluated on the object.
		if member, tail := constraintMemberSegment(rest); member != "" {
			val, found, err := ec.constraintMemberValue(inst, sym, member)
			if !found {
				return Value{}, fmt.Errorf("%w: constraint %s has no member %s", ErrNoSuchFeature, name, member)
			}
			if err != nil {
				return Value{}, err
			}
			return ec.chainMemberValue(val, tail, member)
		}
		val, err := ec.constraintUsageRead(inst, sym)
		if err != nil {
			return Value{}, err
		}
		return ec.chainMemberValue(val, constraintResultTail(rest), name)
	}
	// Read through the feature value so a derived or composite member is materialized
	// on demand rather than read as an empty feature value.
	fv, open, err := ec.memberFeatureValue(inst, name)
	if err != nil {
		return Value{}, err
	}
	// An object read through the model holds, for a feature whose count the model
	// leaves open, no fixed sequence of values.
	if val, ok := ec.openFeatureRead(inst, fv, open, from, name); ok {
		return ec.chainMemberValue(val, rest, name)
	}
	member, err := ec.ctx.readFeatureValue(fv, name)
	if err != nil {
		return Value{}, err
	}
	return ec.chainMemberValue(member, rest, name)
}

// chainOwnFeature continues a chain through a feature the value answers from
// itself rather than from an object it names.
func (ec *EvalContext) chainOwnFeature(value Value, parts []ast.NameSegment, from string) (Value, error) {
	member, err := ec.ownFeature(value, parts[0].Text)
	if err != nil {
		return Value{}, err
	}
	return ec.chainMemberValue(member, parts[1:], from)
}

// chainOverElements reads the rest of a chain from every element of a
// multi-valued member, concatenating the values each contributes.
func (ec *EvalContext) chainOverElements(value Value, parts []ast.NameSegment, from string) (Value, error) {
	var collected, reads []Value
	for _, elem := range elementsOf(value) {
		val, err := ec.chainMemberValue(elem, parts, from)
		if err != nil {
			return Value{}, err
		}
		contributed := elementsOf(val)
		if err := ec.ctx.chargeElements(int64(len(contributed))); err != nil {
			return Value{}, err
		}
		collected = append(collected, contributed...)
		reads = append(reads, val)
	}
	// A member some element leaves undetermined is undetermined over all of them.
	if _, open := undeterminedIn(reads...); open {
		return undeterminedElements(reads), nil
	}
	if len(collected) == 0 {
		if unit, ok := elementUnitOf(reads...); ok {
			return newEmptySequenceOf(unit), nil
		}
	}
	return sequenceOf(collected), nil
}
