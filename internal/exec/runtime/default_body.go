package runtime

import "errors"

// A redefining declaration's body governs over the `default` the redefined declaration
// wrote only for the features it values (`attribute :>> xoffset { attribute :>> num = 0; }`
// over `attribute xoffset : LengthValue default 0 [m]`): a default is used wherever the
// feature is not otherwise given a value (KerML 1.0 §8.3.4.10, FeatureValue::isDefault),
// so it supplies the features of the one object the body leaves unvalued — the ones its
// shape holds unvalued, and the ones a value-held type's shape leaves out (`mRef`).

// holderDefault is the feature value holding inst as its one object when a body governs over
// the fallback default that feature inherits, with the object holding it; ok is false otherwise.
func (ctx *Context) holderDefault(inst *Instance) (owner *Instance, holder *FeatureValue, ok bool) {
	owner, feature := inst.Owner()
	if owner == nil || feature == "" {
		return nil, nil, false
	}
	holder = owner.FeatureValues[feature]
	if holder == nil || holder.Feature == nil || holder.Feature.DefaultValue == nil ||
		!holder.Feature.defaultIsFallback() || !holder.Feature.Scalar() || ctx.valueBinds(holder.Feature) {
		return nil, nil, false
	}
	if held := holder.Value; held.Kind != ValInvalid && (held.Kind != ValInstance || held.Instance != inst.ID) {
		return nil, nil, false
	}
	return owner, holder, true
}

// holderDefaultValue evaluates the default the declaration holding inst governs over, against
// the object holding it; ok is false when no such default applies.
func (ctx *Context) holderDefaultValue(inst *Instance) (Value, bool, error) {
	owner, holder, ok := ctx.holderDefault(inst)
	if !ok {
		return Value{}, false, nil
	}
	val, err := ctx.evalFeatureValueDefault(owner, holder, inst.ownerFeature)
	return val, true, err
}

// holderDefaultMember answers a read of a member inst's shape leaves out — one its value-held
// type carries in the value — from the default its holder governs over; ok is false for a
// member the shape holds, one the type does not declare, or one the default states nothing for.
func (ctx *Context) holderDefaultMember(inst *Instance, name string) (Value, bool, error) {
	if _, held := inst.FeatureValues[name]; held || ctx.model.semantics == nil {
		return Value{}, false, nil
	}
	sym, found := ctx.model.semantics.LookupMember(inst.Type, name)
	if !found || sym == nil || !sym.IsFeature() || !ctx.model.semantics.HeldByValue(sym) {
		return Value{}, false, nil
	}
	whole, ok, err := ctx.holderDefaultValue(inst)
	if !ok || err != nil {
		return Value{}, ok, err
	}
	return ctx.memberOfValue(whole, name)
}

// materializeFromHolderDefault holds, on a feature of inst's shape nothing else values, what
// the default its holder governs over states for it; held is false when none supplies one.
func (inst *Instance) materializeFromHolderDefault(ctx *Context, fv *FeatureValue, name string) (held bool, err error) {
	if _, _, ok := ctx.holderDefault(inst); !ok {
		return false, nil
	}
	supplied := true
	_, err = inst.holdDerived(ctx, fv, name, func() (Value, error) {
		whole, _, err := ctx.holderDefaultValue(inst)
		if err != nil {
			return Value{}, err
		}
		member, ok, err := ctx.memberOfValue(whole, name)
		if err != nil {
			return Value{}, err
		}
		if !ok {
			supplied = false
			return Value{}, errNoDefaultMember
		}
		return member, nil
	})
	if !supplied {
		return false, nil
	}
	return err == nil, err
}

// errNoDefaultMember stops a derivation whose default states nothing for the feature.
var errNoDefaultMember = errors.New("default states no value for the feature")

// holderDefaultSupplies reports a value fv holds that only the default its holder governs
// over could have supplied: materialized, with nothing of its own stating it.
func (ctx *Context) holderDefaultSupplies(inst *Instance, fv *FeatureValue) bool {
	if fv == nil || !fv.Materialized || fv.Written || fv.BindingDerived || fv.HeldValue().Kind == ValInvalid ||
		(fv.Feature != nil && fv.Feature.DefaultValue != nil) {
		return false
	}
	_, _, ok := ctx.holderDefault(inst)
	return ok
}

// memberOfValue is the named feature of a value — an object's feature value holding a
// value, or a library feature a structured value answers — and whether it has one.
func (ctx *Context) memberOfValue(val Value, name string) (Value, bool, error) {
	if inst, ok := ctx.instances[val.Instance]; ok && val.Kind == ValInstance {
		if _, has := inst.FeatureValues[name]; !has {
			return Value{}, false, nil
		}
		fv, err := inst.GetFeatureValue(ctx, name)
		if err != nil {
			return Value{}, true, err
		}
		held := fv.HeldValue()
		if held.Kind == ValInvalid || ctx.HoldsNoValue(held) {
			return Value{}, false, nil
		}
		return held, true, nil
	}
	member, ok, err := ctx.structuredFeature(val, name)
	// A library feature the value does not hold (`isBound` of a scalar quantity) is no member of it.
	if errors.Is(err, errUnevaluableLibraryFunction) {
		return Value{}, false, nil
	}
	return member, ok, err
}
