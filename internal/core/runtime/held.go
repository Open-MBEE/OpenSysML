package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// HeldObject is one object a feature of another object holds, under the segment
// a path names it by: the feature's name, indexed for a collection's element.
type HeldObject struct {
	Feature  string
	Segment  string
	Instance *Instance
}

// HeldObjectsError reports the feature HeldObjects could not read, and why.
type HeldObjectsError struct {
	Feature string
	Err     error
}

func (e *HeldObjectsError) Error() string {
	return fmt.Sprintf("feature %s: %v", e.Feature, e.Err)
}

func (e *HeldObjectsError) Unwrap() error { return e.Err }

// HeldObjects returns the objects inst's object-valued features hold — occurrence
// usages and structured attributes alike — in feature order then collection order,
// materializing lazy ones as reading them does. An object reached through several
// features is listed once, under a scalar feature when it has one. A feature that
// cannot be read is a HeldObjectsError.
func (ctx *Context) HeldObjects(inst *Instance) ([]HeldObject, error) {
	if err := ctx.checkNotDestroyed(inst); err != nil {
		return nil, err
	}
	var out []HeldObject
	var indexedAt []bool
	at := make(map[int64]int)
	read := make(map[*FeatureValue]bool)
	reach := func(feature string, segment string, indexed bool, val Value) {
		id, ok := val.Object()
		if !ok || ctx.HoldsNoValue(val) {
			return
		}
		child, ok := ctx.instances[id]
		if !ok {
			return
		}
		held := HeldObject{Feature: feature, Segment: segment, Instance: child}
		if i, seen := at[id]; seen {
			if indexedAt[i] && !indexed {
				out[i], indexedAt[i] = held, false
			}
			return
		}
		at[id] = len(out)
		out = append(out, held)
		indexedAt = append(indexedAt, indexed)
	}
	for _, of := range ctx.FeaturesOfObject(inst) {
		if of.Name == "" || !(holdsObjects(of.Feature) || ctx.namesStructuredValue(of.Feature.Symbol)) {
			continue
		}
		fv, err := inst.GetFeatureValue(ctx, of.Name)
		if err != nil {
			return nil, &HeldObjectsError{Feature: of.Name, Err: err}
		}
		if fv == nil || read[fv] {
			continue
		}
		read[fv] = true
		segment := lexer.NameText(of.Name)
		if fv.Values.Kind == ValInvalid {
			reach(of.Name, segment, false, fv.Value)
			continue
		}
		for i, element := range elementsOf(fv.Values) {
			reach(of.Name, fmt.Sprintf("%s[%d]", segment, i+1), true, element)
		}
	}
	return out, nil
}

// Types returns the types an object has: the one it was materialized from,
// then the classifiers a behavior has since given it.
func (inst *Instance) Types() []*symbols.Symbol {
	return append([]*symbols.Symbol(nil), inst.types()...)
}

// HeldUnder returns the usage an object's owner holds it as (`Car::wheels` for
// `car.wheels[2]`), or nil for an object held by none.
func (inst *Instance) HeldUnder() *symbols.Symbol {
	if inst.owner == nil {
		return nil
	}
	fv, ok := inst.owner.FeatureValues[inst.ownerFeature]
	if !ok || fv.Feature == nil {
		return nil
	}
	return fv.Feature.Symbol
}
