package runtime

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// variantReference is the value a name resolving to a variant evaluates to: the
// choice itself, which a variation is bound to and compared with.
func variantReference(sym *symbols.Symbol) Value {
	return NewVariantValue(sym, 0)
}

// IsVariationFeature reports whether a feature is a variation point, whose feature value
// holds the variant it is bound to rather than an object of itself.
func (ctx *Context) IsVariationFeature(feat *EffectiveFeature) bool {
	return feat != nil && ctx.model.semantics.IsVariationFeature(feat.Symbol)
}

// variantSegment reads a variant named through the variation feature it belongs
// to, as in `ring.nesting::nestingTrue`, returning it and the segments left.
func (ec *EvalContext) variantSegment(feat *EffectiveFeature, rest []ast.NameSegment) (*symbols.Symbol, []ast.NameSegment, bool) {
	if len(rest) == 0 || feat == nil || !ec.ctx.model.semantics.IsVariationFeature(feat.Symbol) {
		return nil, nil, false
	}
	variant, ok := ec.ctx.model.semantics.VariantOf(feat.Symbol, rest[0].Text)
	if !ok {
		return nil, nil, false
	}
	return variant, rest[1:], true
}

// variantSummary names the variants a variation offers, for a report about a
// selection that is not one of them.
func (ctx *Context) variantSummary(variation *symbols.Symbol) string {
	variants := ctx.model.semantics.VariantsOf(variation)
	if len(variants) == 0 {
		return "it declares no variants"
	}
	names := make([]string, 0, len(variants))
	for _, v := range variants {
		names = append(names, v.Name)
	}
	return "variants: " + strings.Join(names, ", ")
}

// bindVariation resolves what a variation feature is bound to into the value its
// feature value holds: the selected variant's value, or an object of it (SysML v2 §7.20).
// Anything that is not one of the feature's variants is reported.
func (ctx *Context) bindVariation(feat *EffectiveFeature, selection Value, owner int64) (Value, error) {
	name := feat.Name
	switch selection.Kind {
	case ValSequence:
		return ctx.bindOneVariant(feat, selection.Sequence().Elements(), owner)
	case ValSet:
		return ctx.bindOneVariant(feat, selection.Set().Elements(), owner)
	case ValVariant:
	default:
		if ctx.model.semantics.IsVariationFeature(feat.Symbol) {
			return Value{}, fmt.Errorf("%w: variation %s is bound to a %s", ErrNotAVariant, name, selection.Kind)
		}
		return selection, nil
	}

	variant := selection.Variant()
	if !ctx.model.semantics.SelectsVariantOf(feat.Symbol, variant) {
		return Value{}, fmt.Errorf("%w: %s is not a variant of %s (%s)",
			ErrNotAVariant, variant.Name, name, ctx.variantSummary(feat.Symbol))
	}
	ctx.selectVariant(variantSelection{owner: owner, variation: name}, variant.Name)
	return ctx.variantValue(feat.Symbol, variant, owner)
}

// selectVariant records the variant a variation is bound to for routing to consult;
// a probe or transaction rolled back restores the selection as it was.
func (ctx *Context) selectVariant(selection variantSelection, variant string) {
	prior, selected := ctx.selectedVariants[selection]
	if selected && prior == variant {
		return
	}
	ctx.noteProbeUndo(func() {
		if selected {
			ctx.selectedVariants[selection] = prior
		} else {
			delete(ctx.selectedVariants, selection)
		}
	})
	ctx.selectedVariants[selection] = variant
}

// bindVariationOf binds a value read from a feature's declaration when that
// feature is a variation, so what a legal selection is does not depend on
// whether the variation is read through an object or through its declaration.
func (ec *EvalContext) bindVariationOf(sym *symbols.Symbol, val Value) (Value, error) {
	if !ec.ctx.model.semantics.IsVariationFeature(sym) {
		return val, nil
	}
	owner := int64(0)
	if ec.self != nil {
		owner = ec.self.ID
	}
	return ec.ctx.bindVariation(&EffectiveFeature{Name: sym.Name, Symbol: sym}, val, owner)
}

// bindOneVariant binds a variation bound to a collection: exactly one variant
// may be selected, so selecting several, or anything that is not a variant, is
// reported rather than resolved.
func (ctx *Context) bindOneVariant(feat *EffectiveFeature, elements []Value, owner int64) (Value, error) {
	var selected []Value
	for _, el := range elements {
		if el.Kind != ValVariant {
			return Value{}, fmt.Errorf("%w: variation %s is bound to a collection holding a %s (%s)",
				ErrNotAVariant, feat.Name, el.Kind, ctx.variantSummary(feat.Symbol))
		}
		selected = append(selected, el)
	}
	switch {
	case len(selected) > 1:
		names := make([]string, 0, len(selected))
		for _, el := range selected {
			names = append(names, el.Variant().Name)
		}
		return Value{}, fmt.Errorf("%w: variation %s selects %d variants (%s)",
			ErrMultipleVariants, feat.Name, len(selected), strings.Join(names, ", "))
	case len(selected) == 1:
		return ctx.bindVariation(feat, selected[0], owner)
	default:
		return Value{}, fmt.Errorf("%w: variation %s is bound to a collection naming no variant (%s)",
			ErrNotAVariant, feat.Name, ctx.variantSummary(feat.Symbol))
	}
}

// variantValue materializes a selected variant: the value it declares, or an
// object of it carrying its nested values and connections. The object belongs to
// the owner that selected it, materialized once for that owner.
func (ctx *Context) variantValue(variation, variant *symbols.Symbol, owner int64) (Value, error) {
	if value := semantics.VariantValue(variant); value != nil {
		ec := NewEvalContext(ctx, declScope(variant))
		val, err := ec.Eval(value)
		if err != nil {
			return Value{}, fmt.Errorf("variant %s: %w", variant.Name, err)
		}
		return val, nil
	}
	key := variantObject{owner: owner, variation: variation, variant: variant}
	if id, ok := ctx.variantObjects[key]; ok {
		if _, live := ctx.instances[id]; live {
			return NewVariantValue(variant, id), nil
		}
	}
	var inst *Instance
	err := ctx.variantInstance(variant, owner, func(created *Instance) {
		inst = created
		ctx.variantObjects[key] = inst.ID
	})
	if err != nil {
		return Value{}, fmt.Errorf("variant %s: %w", variant.Name, err)
	}
	return NewVariantValue(variant, inst.ID), nil
}

// variantInstance builds the object a selected variant stands for. A variant
// that is itself a connector — `variant interface engagementRingToBandConnected
// connect engagementRing.ringPort to band.ringPort` — is the connection the
// selection realizes, so it is materialized as a connector of the object that
// selected it, with its ends attached to that object's features. A variant of
// any other kind is an ordinary object of itself. keep receives the object once created.
func (ctx *Context) variantInstance(variant *symbols.Symbol, owner int64, keep func(*Instance)) error {
	if !ctx.model.semantics.IsConnectorUsage(variant) {
		inst, err := ctx.instantiateAs(variant, 0)
		if err != nil {
			return err
		}
		keep(inst)
		return nil
	}
	ownerInst, ok := ctx.Instance(owner)
	if !ok {
		return fmt.Errorf("%w: %s connects features of the object selecting it, and no object selected it",
			ErrConnectorEnd, variant.Name)
	}
	return ctx.materializeConnector(ownerInst, variant, ctx.variantConnectorBase(variant), keep)
}

// variantConnectorBase returns the type an object of a variant connector is
// materialized from: the definition the variant names, else the one its
// variation names, else the variant itself when neither is typed — a
// `variant interface … connect …` under an untyped `variation interface` is
// implicitly typed, exactly as a standalone untyped connector usage is.
func (ctx *Context) variantConnectorBase(variant *symbols.Symbol) *symbols.Symbol {
	if base := ctx.CompositeTypeOf(&EffectiveFeature{Name: variant.Name, Symbol: variant}); base != nil {
		return base
	}
	return variant
}

// variantAsValue resolves a variant to the value it declares, so comparing a
// value with a variant compares what the variant stands for. A variant
// declaring no value is identified by the selection itself.
func (ctx *Context) variantAsValue(v Value) (Value, error) {
	if v.Kind != ValVariant || v.Variant() == nil {
		return v, nil
	}
	if semantics.VariantValue(v.Variant()) == nil {
		return v, nil
	}
	// A variant reached this way declares a value, so no object of it is needed.
	return ctx.variantValue(nil, v.Variant(), 0)
}

// variantObject keys the object a variant stands for by the selection that made
// it: two owners, or two variation points read without an owner, each have their
// own object.
type variantObject struct {
	owner     int64
	variation *symbols.Symbol
	variant   *symbols.Symbol
}
