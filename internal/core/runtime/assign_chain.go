package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
)

// assignThroughChain writes value to the feature a chained target names, on the
// object its chain reaches: `assign a.b := v` writes feature `b` of the object
// `a` reaches, resolved in the scope the statement was written in. This is
// Actions::AssignmentAction, which "updates the accessedFeature of its target
// Occurrence with the given replacementValues".
func assignThroughChain(ec *EvalContext, host string, s lower.Assign, value Value) error {
	if err := writeThroughChain(ec, s.Chain, s.Target, value); err != nil {
		return fmt.Errorf("%s: assignment to %s: %w", host, s.Chain.Text, err)
	}
	return nil
}

// writeThroughChain writes value to feature of the object chain reaches, as an
// assignment or a binding at a node's pin does.
func writeThroughChain(ec *EvalContext, chain *lower.AssignTarget, feature string, value Value) error {
	target, err := ec.chainCarrier(chain)
	if err != nil {
		return err
	}
	if _, ok := target.FeatureValues[feature]; !ok {
		return fmt.Errorf("%w: object #%d (%s) has no feature %s",
			ErrNoSuchFeature, target.ID, symbolText(target.Type), feature)
	}
	// Written through the object itself, so the value is multiplicity-checked and
	// seen by every feature reaching that object, as a direct write is.
	if err := target.SetFeatureValue(ec.ctx, feature, value); err != nil {
		return err
	}
	ec.ctx.noteObjectWrite(target, feature, value)
	return nil
}

// chainCarrier walks a chained target's steps to the object whose feature the
// assignment writes, reading each step as a chain read does so a write and a
// read of one path reach one object. A step holding no object, or holding more
// than one, names no object to write on and is reported.
func (ec *EvalContext) chainCarrier(target *lower.AssignTarget) (*Instance, error) {
	value, err := ec.chainRoot(target.Base)
	if err != nil {
		return nil, err
	}
	reached := lower.FeaturePath(target.Base)
	for _, step := range target.Steps {
		if _, err := ec.chainObject(value, reached); err != nil {
			return nil, err
		}
		if value, err = ec.chainMemberValue(value, []ast.NameSegment{{Text: step}}, reached); err != nil {
			return nil, err
		}
		reached = step
	}
	return ec.chainObject(value, reached)
}

// chainRoot evaluates the node a chained target starts from, as the object it
// names: a part names an occurrence rather than a value, and a calc usage names
// an evaluation, which holds no object a write could reach.
func (ec *EvalContext) chainRoot(base ast.Node) (Value, error) {
	if sym, ok := ec.calcUsageOperand(base); ok {
		return Value{}, fmt.Errorf("%w: calc usage %s computes output features and holds no object",
			ErrTypeMismatch, symbolText(sym))
	}
	if sym, ok := ec.occurrenceOperand(base); ok {
		// A collection reads as its objects, which the step after it refuses as several.
		if ec.ctx.namesObjects(sym) {
			return ec.ctx.denotedValue(sym)
		}
		inst, err := ec.ctx.occurrenceOf(sym)
		if err != nil {
			return Value{}, fmt.Errorf("usage %s: %w", symbolText(sym), err)
		}
		return Value{Kind: ValInstance, Instance: inst.ID}, nil
	}
	return ec.Eval(base)
}

// chainObject is the one object a step of a chained target reaches, which the
// step after it walks from and the last step writes on. named names the step for
// the diagnostic.
func (ec *EvalContext) chainObject(value Value, named string) (*Instance, error) {
	if literal := value.EnumerationLiteral(); literal != nil {
		return ec.ctx.enumLiteralObject(literal)
	}
	switch value.Kind {
	case ValNull, ValInvalid:
		return nil, fmt.Errorf("%w: %s", ErrUninitializedFeatureValue, named)
	case ValSequence, ValSet:
		return nil, fmt.Errorf("%w: %s holds %s, and a write reaches one object",
			ErrTypeMismatch, named, describeValue(value))
	}
	id, isObject := value.Object()
	if !isObject {
		return nil, fmt.Errorf("%w: %s is %s, not an object to write a feature of",
			ErrTypeMismatch, named, describeValue(value))
	}
	inst, ok := ec.ctx.instances[id]
	if !ok {
		return nil, fmt.Errorf("%w: object #%d of %s no longer exists", ErrUnresolvedReference, id, named)
	}
	return inst, nil
}
