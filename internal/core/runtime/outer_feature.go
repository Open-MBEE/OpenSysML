package runtime

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// outerFeatureValue reads the resolved feature from the nearest owner carrying it: a nested usage
// naming `Rectangle::length` or a sibling reads the enclosing object's value (KerML 1.0 §7.4.8.1).
func (ec *EvalContext) outerFeatureValue(sym *symbols.Symbol) (Value, bool, error) {
	if sym == nil || !semantics.IsShapeFeature(sym) {
		return Value{}, false, nil
	}
	for inst := ec.self; inst != nil; inst, _ = inst.Owner() {
		name, ok := ec.ctx.featureDenoting(inst, sym)
		if !ok {
			continue
		}
		fv, err := inst.GetFeatureValue(ec.ctx, name)
		if err != nil {
			return Value{}, true, err
		}
		val, err := ec.ctx.readFeatureValue(fv, name)
		if err != nil {
			return val, true, err
		}
		// An object the feature holds is read as what it denotes, as a chain reads it.
		if held, ok := ec.ctx.instances[val.Instance]; ok && val.Kind == ValInstance {
			val, err = ec.ctx.objectValue(held)
		}
		return val, true, err
	}
	return Value{}, false, nil
}

// featureDenoting names the feature value of inst whose feature is sym or redefines it, skipping
// one whose own default is being evaluated so that default reads an outer object's value.
func (ctx *Context) featureDenoting(inst *Instance, sym *symbols.Symbol) (string, bool) {
	for _, typ := range inst.types() {
		name, ok := ctx.denotedFeature(typ, sym)
		if !ok {
			continue
		}
		if _, ok := inst.FeatureValues[name]; !ok ||
			ctx.derivingFeatureValues[featureValueRef{instance: inst.ID, feature: name}] {
			continue
		}
		return name, true
	}
	return "", false
}

// denotedFeature names the feature of typ that sym denotes on an object of typ.
func (ctx *Context) denotedFeature(typ, sym *symbols.Symbol) (string, bool) {
	index, ok := ctx.model.denotedFeatures[typ]
	if !ok {
		index = make(map[*symbols.Symbol]string)
		for _, feat := range ctx.FeaturesOf(typ) {
			if feat.Symbol == nil {
				continue
			}
			if _, seen := index[feat.Symbol]; !seen {
				index[feat.Symbol] = feat.Name
			}
			for _, redefined := range ctx.redefinedFeatures(feat.Symbol, typ) {
				if _, seen := index[redefined]; !seen {
					index[redefined] = feat.Name
				}
			}
		}
		ctx.model.denotedFeatures[typ] = index
	}
	name, ok := index[sym]
	return name, ok
}

// types answers the type an object was created as followed by the classifiers
// bindings have since added to it.
func (inst *Instance) types() []*symbols.Symbol {
	if inst.Type == nil {
		return inst.classifiers
	}
	return append([]*symbols.Symbol{inst.Type}, inst.classifiers...)
}

// ObjectFeature is a feature of an object under one of its names: the declaration its
// feature value reads, or its type's when it holds no value under the name.
type ObjectFeature struct {
	Name    string
	Feature *EffectiveFeature
}

// FeaturesOfObject returns the features of an object under every type of it: the declared
// type's, then those each classifier adds, in declaration order, each name once.
func (ctx *Context) FeaturesOfObject(inst *Instance) []ObjectFeature {
	types := inst.types()
	var out []ObjectFeature
	var seen map[string]bool
	if len(types) > 1 {
		seen = make(map[string]bool)
	}
	for _, typ := range types {
		features := ctx.FeaturesOf(typ)
		if out == nil {
			out = make([]ObjectFeature, 0, len(features))
		}
		for i := range features {
			name := features[i].Name
			if seen[name] {
				continue
			}
			if seen != nil {
				seen[name] = true
			}
			feat := &features[i]
			if fv := inst.FeatureValues[name]; fv != nil && fv.Feature != nil {
				feat = fv.Feature
			}
			out = append(out, ObjectFeature{Name: name, Feature: feat})
		}
	}
	return out
}
